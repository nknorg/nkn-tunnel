package tunnel

import (
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/nknorg/ncp-go"
	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	"github.com/nknorg/nkngomobile"
	"github.com/patrickmn/go-cache"
)

type nknDialer interface {
	Addr() net.Addr
	DialWithConfig(addr string, config *nkn.DialConfig) (*ncp.Session, error)
	DialUDPWithConfig(remoteAddr string, config *nkn.DialConfig) (*ts.UdpSession, error)
	Close() error
}

type nknListener interface {
	Listen(addrsRe *nkngomobile.StringArray) error
}

// Tunnel is the tunnel client struct.
type Tunnel struct {
	from        string
	to          string
	fromNKN     bool
	toNKN       bool
	config      *Config
	dialer      nknDialer
	listeners   []net.Listener
	multiClient *nkn.MultiClient
	tsClient    *ts.TunaSessionClient

	lock     sync.RWMutex
	isClosed bool

	udpLock      sync.RWMutex
	udpConnCache *cache.Cache
}

// NewTunnel creates a Tunnel client with given options.
func NewTunnel(account *nkn.Account, identifier, from, to string, tuna bool, config *Config) (*Tunnel, error) {
	tunnels, err := NewTunnels(account, identifier, []string{from}, []string{to}, tuna, config)
	if err != nil {
		return nil, err
	}

	return tunnels[0], nil
}

// NewTunnels creates Tunnel clients with given options.
func NewTunnels(account *nkn.Account, identifier string, from, to []string, tuna bool, config *Config) ([]*Tunnel, error) {
	if len(from) != len(to) || len(from) == 0 {
		return nil, errors.New("from should have same length as to")
	}

	config, err := MergedConfig(config)
	if err != nil {
		return nil, err
	}
	if config.UDP && !tuna {
		return nil, ErrUDPNotSupported
	}

	udpConnExpired := cache.NoExpiration
	if config.UDPIdleTime > 0 {
		udpConnExpired = time.Duration(config.UDPIdleTime) * time.Second
	}

	fromNKN := false
	for _, f := range from {
		fromNKN = (len(f) == 0 || strings.ToLower(f) == "nkn")
		if fromNKN {
			break
		}
	}
	if fromNKN && len(from) > 1 {
		return nil, errors.New("multiple tunnels is not supported when from NKN")
	}

	var m *nkn.MultiClient
	var c *ts.TunaSessionClient
	var dialer nknDialer

	m, err = nkn.NewMultiClient(account, identifier, config.NumSubClients, config.OriginalClient, config.ClientConfig)
	if err != nil {
		return nil, err
	}

	<-m.OnConnect.C
	dialer = newMultiClientDialer(m)

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

	tunnels := make([]*Tunnel, 0)
	for i, f := range from {
		toNKN := !strings.Contains(to[i], ":")
		listeners := make([]net.Listener, 0, 2)

		if fromNKN {
			if tuna {
				if config.TunaNode != nil {
					c.SetTunaNode(config.TunaNode)
				}
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

			f = m.Addr().String()
		} else {
			listener, err := net.Listen("tcp", f)
			if err != nil {
				return nil, err
			}
			listeners = append(listeners, listener)
		}

		log.Println("Listening at", f)

		t := &Tunnel{
			from:         f,
			to:           to[i],
			fromNKN:      fromNKN,
			toNKN:        toNKN,
			config:       config,
			dialer:       dialer,
			listeners:    listeners,
			multiClient:  m,
			tsClient:     c,
			udpConnCache: cache.New(udpConnExpired, udpConnExpired),
		}
		tunnels = append(tunnels, t)
	}

	return tunnels, nil
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

// MultiClient returns the NKN multiclient that tunnel creates and uses.
func (t *Tunnel) MultiClient() *nkn.MultiClient {
	return t.multiClient
}

// TunaSessionClient returns the tuna session client that tunnel creates and
// uses. It is not nil only if tunnel is created with tuna == true.
func (t *Tunnel) TunaSessionClient() *ts.TunaSessionClient {
	return t.tsClient
}

// TunaPubAddrs returns the public node info of tuna listeners. Returns nil if
// there is no tuna listener.
func (t *Tunnel) TunaPubAddrs() *ts.PubAddrs {
	for _, listener := range t.listeners {
		if c, ok := listener.(*ts.TunaSessionClient); ok {
			return c.GetPubAddrs()
		}
	}
	return nil
}

// SetAcceptAddrs updates the accept address regex for incoming sessions.
// Tunnel will accept sessions from address that matches any of the given
// regular expressions. If addrsRe is nil, any address will be accepted. Each
// function call will overwrite previous accept addresses.
func (t *Tunnel) SetAcceptAddrs(addrsRe *nkngomobile.StringArray) error {
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
		return t.dialer.DialWithConfig(addr, t.config.DialConfig)
	}
	var dialTimeout time.Duration
	if t.config.DialConfig != nil {
		dialTimeout = time.Duration(t.config.DialConfig.DialTimeout) * time.Millisecond
	}
	return net.DialTimeout("tcp", addr, dialTimeout)
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
						fromConn.Close()
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

	if t.config.UDP {
		fromUDPConn, err := t.listenUDP()
		if err != nil {
			return err
		}
		go t.udpPipe(fromUDPConn)
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
