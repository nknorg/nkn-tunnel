package tunnel

import (
	"encoding/hex"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	nknsdk "github.com/nknorg/nkn-sdk-go"
	session "github.com/nknorg/nkn-tuna-session"
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
}

func NewTunnel(numClients int, seed []byte, identifier, from, to string, tuna bool) (*Tunnel, error) {
	fromNKN := strings.ToLower(from) == "nkn"
	toNKN := !strings.Contains(to, ":")
	var m *nknsdk.MultiClient
	var c *session.TunaSessionClient
	var err error
	var dialer nknDialer

	if fromNKN || toNKN {
		account, err := nknsdk.NewAccount(seed)
		if err != nil {
			return nil, err
		}

		log.Println("Seed:", hex.EncodeToString(account.PrivateKey[:32]))

		m, err = nknsdk.NewMultiClient(account, identifier, numClients, false, &nknsdk.ClientConfig{ConnectRetries: 1})
		if err != nil {
			return nil, err
		}

		dialer = m

		if tuna {
			wallet, err := nknsdk.NewWallet(account, nil)
			if err != nil {
				return nil, err
			}

			c, err = session.NewTunaSessionClient(account, m, wallet, nil)
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

		go func(fromConn net.Conn) {
			toConn, err := t.dial(t.to)
			if err != nil {
				log.Println(err)
				return
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
