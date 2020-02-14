package tunnel

import (
	"encoding/hex"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	nkn "github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
)

type nknDialer interface {
	Dial(addr string) (net.Conn, error)
}

type Tunnel struct {
	from      string
	to        string
	fromNKN   bool
	toNKN     bool
	nknDialer nknDialer
	listener  net.Listener
	verbose   bool
}

func NewTunnel(numClients int, seed []byte, identifier, from, to string, tuna, verbose bool) (*Tunnel, error) {
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

		m, err = nkn.NewMultiClient(account, identifier, numClients, false, &nkn.ClientConfig{ConnectRetries: 1})
		if err != nil {
			return nil, err
		}

		dialer = m

		if tuna {
			wallet, err := nkn.NewWallet(account, nil)
			if err != nil {
				return nil, err
			}

			c, err = ts.NewTunaSessionClient(account, m, wallet, &ts.Config{NumTunaListeners: numClients})
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
	} else {
		return net.Dial("tcp", addr)
	}
}

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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(a, b)
	}()
	go func() {
		defer wg.Done()
		io.Copy(b, a)
	}()
	wg.Wait()
	a.Close()
	b.Close()
}
