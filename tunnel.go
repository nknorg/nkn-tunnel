package tunnel

import (
	"encoding/hex"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/nknorg/ncp-go"
	nkn "github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
)

type nknDialer interface {
	Dial(addr string) (net.Conn, error)
	Close() error
}

// Tunnel is the tunnel client struct.
type Tunnel struct {
	from      string
	to        string
	fromNKN   bool
	toNKN     bool
	dialer    nknDialer
	listeners []net.Listener
	verbose   bool

	lock     sync.RWMutex
	isClosed bool
}

// NewTunnel creates a Tunnel client with given options.
func NewTunnel(numClients int, seed []byte, identifier, from, to string, sessionConfig *ncp.Config, tuna bool, tsConfig *ts.Config, verbose bool) (*Tunnel, error) {
	fromNKN := strings.ToLower(from) == "nkn"
	toNKN := !strings.Contains(to, ":")
	var m *nkn.MultiClient
	var c *ts.TunaSessionClient
	var err error
	var dialer nknDialer

	if fromNKN || toNKN {
		account, err := nkn.NewAccount(seed)
		if err != nil {
			return nil, err
		}

		log.Println("Seed:", hex.EncodeToString(account.PrivateKey[:32]))

		m, err = nkn.NewMultiClient(account, identifier, numClients, false, &nkn.ClientConfig{SessionConfig: sessionConfig})
		if err != nil {
			return nil, err
		}

		<-m.OnConnect.C

		dialer = m

		if tuna {
			wallet, err := nkn.NewWallet(account, nil)
			if err != nil {
				return nil, err
			}

			if tsConfig != nil {
				tsConfigCopy := *tsConfig
				tsConfigCopy.NumTunaListeners = numClients
				tsConfigCopy.SessionConfig = sessionConfig
				tsConfig = &tsConfigCopy
			}

			c, err = ts.NewTunaSessionClient(account, m, wallet, tsConfig)
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
			err = c.Listen(nil)
			if err != nil {
				return nil, err
			}
		}
		listeners = append(listeners, m)
		err = m.Listen(nil)
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

	log.Println("Listening at", from)

	t := &Tunnel{
		from:      from,
		to:        to,
		fromNKN:   fromNKN,
		toNKN:     toNKN,
		dialer:    dialer,
		listeners: listeners,
		verbose:   verbose,
	}

	return t, nil
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

				if t.verbose {
					log.Println("Accept from", fromConn.RemoteAddr())
				}

				go func(fromConn net.Conn) {
					toConn, err := t.dial(t.to)
					if err != nil {
						log.Println(err)
						return
					}

					if t.verbose {
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

func (t *Tunnel) IsClosed() bool {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.isClosed
}

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
