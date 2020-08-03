package tunnel

import (
	"encoding/hex"
	"io"
	"log"
	"net"
	"strings"

	"github.com/nknorg/ncp-go"
	nkn "github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
)

type nknDialer interface {
	Dial(addr string) (net.Conn, error)
}

// Tunnel is the tunnel client struct.
type Tunnel struct {
	from      string
	to        string
	fromNKN   bool
	toNKN     bool
	nknDialer nknDialer
	listener  net.Listener
	verbose   bool
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

	var listener net.Listener

	if fromNKN {
		if tuna {
			listener = c
			err = c.Listen(nil)
		} else {
			listener = m
			err = m.Listen(nil)
		}
		if err != nil {
			return nil, err
		}
		from = listener.Addr().String()
	} else {
		listener, err = net.Listen("tcp", from)
		if err != nil {
			return nil, err
		}
	}

	log.Println("Listening at", from)

	t := &Tunnel{
		from:      from,
		to:        to,
		fromNKN:   fromNKN,
		toNKN:     toNKN,
		nknDialer: dialer,
		listener:  listener,
		verbose:   verbose,
	}

	return t, nil
}

func (t *Tunnel) dial(addr string) (net.Conn, error) {
	if t.toNKN {
		return t.nknDialer.Dial(addr)
	}
	return net.Dial("tcp", addr)
}

// Start starts the tunnel and will return on error.
func (t *Tunnel) Start() error {
	for {
		fromConn, err := t.listener.Accept()
		if err != nil {
			return err
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
