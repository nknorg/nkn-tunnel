package tunnel

import (
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	nkn "github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
)

type nknDialer interface {
	Addr() net.Addr
	Dial(addr string) (net.Conn, error)
	Close() error
}

type nknListener interface {
	Listen(addrsRe *nkn.StringArray) error
}

// Tunnel is the tunnel client struct.
type Tunnel struct {
	from      string
	to        string
	fromNKN   bool
	toNKN     bool
	config    *Config
	dialer    nknDialer
	listeners []net.Listener

	lock     sync.RWMutex
	isClosed bool
}

// NewTunnel creates a Tunnel client with given options.
func NewTunnel(account *nkn.Account, identifier, from, to string, tuna bool, config *Config) (*Tunnel, error) {
	config, err := MergedConfig(config)
	if err != nil {
		return nil, err
	}

	fromNKN := len(from) == 0 || strings.ToLower(from) == "nkn"
	toNKN := !strings.Contains(to, ":")
	var m *nkn.MultiClient
	var c *ts.TunaSessionClient
	var dialer nknDialer

	if fromNKN || toNKN {
		m, err = nkn.NewMultiClient(account, identifier, config.NumSubClients, config.OriginalClient, config.ClientConfig)
		if err != nil {
			return nil, err
		}

		<-m.OnConnect.C

		dialer = m

		if tuna {
			wallet, err := nkn.NewWallet(account, config.WalletConfig)
			if err != nil {
				return nil, err
			}

			c, err = ts.NewTunaSessionClient(account, m, wallet, config.TunaSessionConfig)
			if err != nil {
				return nil, err
			}

			dialer = c
		}
	}

	listeners := make([]net.Listener, 0, 2)

	if fromNKN {
		if tuna {
			listeners = append(listeners, c)
			err = c.Listen(config.AcceptAddrs)
			if err != nil {
				return nil, err
			}
		}
		listeners = append(listeners, m)
		err = m.Listen(config.AcceptAddrs)
		if err != nil {
			return nil, err
		}
		from = m.Addr().String()
	} else {
		listener, err := net.Listen("tcp", from)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, listener)
	}

	if config.Verbose {
		log.Println("Listening at", from)
	}

	t := &Tunnel{
		from:      from,
		to:        to,
		fromNKN:   fromNKN,
		toNKN:     toNKN,
		config:    config,
		dialer:    dialer,
		listeners: listeners,
	}

	return t, nil
}

// FromAddr returns the tunnel listening address.
func (t *Tunnel) FromAddr() string {
	return t.from
}

// ToAddr returns the tunnel dialing address.
func (t *Tunnel) ToAddr() string {
	return t.to
}

// Addr returns the tunnel NKN address.
func (t *Tunnel) Addr() net.Addr {
	return t.dialer.Addr()
}

// SetAcceptAddrs updates the accept address regex for incoming sessions.
// Tunnel will accept sessions from address that matches any of the given
// regular expressions. If addrsRe is nil, any address will be accepted. Each
// function call will overwrite previous accept addresses.
func (t *Tunnel) SetAcceptAddrs(addrsRe *nkn.StringArray) error {
	if t.fromNKN {
		for _, listener := range t.listeners {
			err := listener.(nknListener).Listen(addrsRe)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tunnel) dial(addr string) (net.Conn, error) {
	if t.toNKN {
		return t.dialer.Dial(addr)
	}
	return net.Dial("tcp", addr)
}

// Start starts the tunnel and will return on error.
func (t *Tunnel) Start() error {
	errChan := make(chan error, 2)
	for _, listener := range t.listeners {
		go func(listener net.Listener) {
			for {
				fromConn, err := listener.Accept()
				if err != nil {
					errChan <- err
					return
				}

				if t.config.Verbose {
					log.Println("Accept from", fromConn.RemoteAddr())
				}

				go func(fromConn net.Conn) {
					toConn, err := t.dial(t.to)
					if err != nil {
						log.Println(err)
						return
					}

					if t.config.Verbose {
						log.Println("Dial to", toConn.RemoteAddr())
					}

					pipe(fromConn, toConn)
				}(fromConn)
			}
		}(listener)
	}

	err := <-errChan

	if t.IsClosed() {
		return nil
	}

	t.Close()

	return err
}

// IsClosed returns whether the tunnel is closed.
func (t *Tunnel) IsClosed() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.isClosed
}

// Close will close the tunnel.
func (t *Tunnel) Close() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.isClosed {
		return nil
	}

	var errs error

	err := t.dialer.Close()
	if err != nil {
		errs = multierror.Append(errs, err)
	}

	for _, listener := range t.listeners {
		err = listener.Close()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	t.isClosed = true

	return errs
}

func pipe(a, b net.Conn) {
	go func() {
		io.Copy(a, b)
		a.Close()
	}()
	go func() {
		io.Copy(b, a)
		b.Close()
	}()
}
