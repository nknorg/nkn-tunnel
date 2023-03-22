package tunnel

import (
	"github.com/nknorg/tuna"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/nknorg/ncp-go"
	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	"github.com/nknorg/nkngomobile"
)

type nknDialer interface {
	Addr() net.Addr
	Dial(addr string) (net.Conn, error)
	DialUDP(remoteAddr string) (*tuna.EncryptUDPConn, error)
	DialWithConfig(addr string, config *nkn.DialConfig) (*ncp.Session, error)
	DialUDPWithConfig(remoteAddr string, config *nkn.DialConfig) (*tuna.EncryptUDPConn, error)
	Close() error
}

type nknListener interface {
	Listen(addrsRe *nkngomobile.StringArray) error
}

type multiClientDialer struct {
	c *nkn.MultiClient
}

func newMultiClientDialer(client *nkn.MultiClient) *multiClientDialer {
	return &multiClientDialer{c: client}
}

func (m *multiClientDialer) Addr() net.Addr {
	return m.c.Addr()
}

func (m *multiClientDialer) Dial(addr string) (net.Conn, error) {
	return m.c.Dial(addr)
}

func (m *multiClientDialer) DialUDP(remoteAddr string) (*tuna.EncryptUDPConn, error) {
	return nil, nil
}

func (m *multiClientDialer) DialWithConfig(addr string, config *nkn.DialConfig) (*ncp.Session, error) {
	return m.c.DialWithConfig(addr, config)
}

func (m *multiClientDialer) DialUDPWithConfig(remoteAddr string, config *nkn.DialConfig) (*tuna.EncryptUDPConn, error) {
	return nil, nil
}

func (m *multiClientDialer) Close() error {
	return m.c.Close()
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
	fromUDPConn *tuna.EncryptUDPConn
	toUDPConn   *tuna.EncryptUDPConn

	lock     sync.RWMutex
	udpLock  sync.RWMutex
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
	}

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
		from:        from,
		to:          to,
		fromNKN:     fromNKN,
		toNKN:       toNKN,
		config:      config,
		dialer:      dialer,
		listeners:   listeners,
		multiClient: m,
		tsClient:    c,
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

// SetAcceptAddrs updates to accept address regex for incoming sessions.
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

func (t *Tunnel) dialUDP(addr string) (*tuna.EncryptUDPConn, error) {
	if t.toNKN {
		conn, err := t.dialer.DialUDPWithConfig(addr, t.config.DialConfig)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}
	udpAddr := net.UDPAddr{IP: net.ParseIP(host), Port: port}
	conn, err := net.DialUDP("udp", nil, &udpAddr)
	if err != nil {
		return nil, err
	}
	return tuna.NewEncryptUDPConn(conn), nil
}

// Start starts the tunnel and will return on error.
func (t *Tunnel) Start() error {
	errChan := make(chan error, 2)
	remoteAddr := new(net.UDPAddr)
	var err error

	for _, listener := range t.listeners {
		go func(listener net.Listener) {
			for {
				fromConn, err := listener.Accept()
				if err != nil {
					errChan <- err
					return
				}

				log.Println("Accept from", fromConn.RemoteAddr())

				go func(fromConn net.Conn) {
					toConn, err := t.dial(t.to)
					if err != nil {
						log.Println(err)
						fromConn.Close()
						return
					}

					log.Println("Dial to", toConn.RemoteAddr())

					pipe(fromConn, toConn)
				}(fromConn)
			}
		}(listener)
	}

	if t.config.Udp {
		var fromUDPConn tuna.UDPConn
		if t.fromNKN {
			if c, ok := t.listeners[0].(*ts.TunaSessionClient); ok {
				fromUDPConn, err = c.ListenUDP(t.config.AcceptAddrs)
				if err != nil {
					return err
				}
			}
		} else {
			_, portStr, err := net.SplitHostPort(t.listeners[0].Addr().String())
			if err != nil {
				return err
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return err
			}
			udpAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
			fromUDPConn, err = net.ListenUDP("udp", udpAddr)
			if err != nil {
				return err
			}
		}

		toUDPConn, err := t.GetToUDPConn(true)
		if err != nil {
			return err
		}

		go func() {
			msg := make([]byte, tuna.MaxUDPBufferSize)
			var n int
			for {
				n, remoteAddr, err = fromUDPConn.ReadFromUDP(msg)
				if err != nil {
					log.Println("readFromUDP err:", err)
					continue
				}
				n, _, err = toUDPConn.WriteMsgUDP(msg[:n], nil, nil)
				if err != nil {
					log.Println("writeMsgUDP err:", err)
					toUDPConn, err = t.GetToUDPConn(true)
					if err != nil {
						log.Println("GetToUDPConn err:", err)
					}
					continue
				}
			}
		}()

		go func() {
			buf := make([]byte, tuna.MaxUDPBufferSize)
			for {
				toUDPConn, err = t.GetToUDPConn(false)
				if err != nil {
					return
				}
				n, _, err := toUDPConn.ReadFromUDP(buf[:])
				if err != nil {
					log.Println("readFromUDP err:", err)
					toUDPConn, err = t.GetToUDPConn(true)
					if err != nil {
						log.Println("GetToUDPConn err:", err)
					}
					continue
				}
				n, _, err = fromUDPConn.WriteMsgUDP(buf[:n], nil, remoteAddr)
				if err != nil {
					log.Println("writeMsgUDP err:", err)
					continue
				}
			}
		}()
	}

	err = <-errChan

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

func (t *Tunnel) GetToUDPConn(force bool) (*tuna.EncryptUDPConn, error) {
	t.udpLock.Lock()
	defer t.udpLock.Unlock()
	if force {
		conn, err := t.dialUDP(t.to)
		if err != nil {
			return nil, err
		}
		t.toUDPConn = conn
	}
	return t.toUDPConn, nil
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
