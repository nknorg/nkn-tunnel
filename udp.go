package tunnel

import (
	"errors"
	"log"
	"net"

	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	"github.com/nknorg/tuna"
	"github.com/patrickmn/go-cache"
)

var (
	ErrUDPNotSupported = errors.New("UDP is only supported in tuna mode")
)

// Generic interface for UDP conneciton, compatabile to net.UDPConn, nkn-tuna-session.
type udpConn interface {
	ReadFrom(b []byte) (n int, addr net.Addr, err error)
	WriteTo(b []byte, addr net.Addr) (n int, err error)
	Close() error
}

type multiClientDialer struct {
	*nkn.MultiClient
}

func newMultiClientDialer(client *nkn.MultiClient) *multiClientDialer {
	return &multiClientDialer{client}
}

func (m *multiClientDialer) DialUDPWithConfig(remoteAddr string, config *nkn.DialConfig) (*ts.UdpSession, error) {
	return nil, ErrUDPNotSupported
}

func (t *Tunnel) dialUDP(addr string) (udpConn, error) {
	if t.toNKN {
		udpSess, err := t.dialer.DialUDPWithConfig(addr, t.config.DialConfig)
		if err != nil {
			return nil, err
		}
		return udpSess, nil
	}

	a, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, a)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (t *Tunnel) getToUDPConn(from net.Addr) (udpConn, bool, error) {
	t.udpLock.Lock()
	defer t.udpLock.Unlock()

	toUDPConn, found := t.udpConnCache.Get(from.String())
	if found {
		return toUDPConn.(udpConn), false, nil
	}

	conn, err := t.dialUDP(t.to)
	if err != nil {
		return nil, false, err
	}
	t.udpConnCache.Set(from.String(), conn, cache.DefaultExpiration)

	return conn, true, nil
}

func (t *Tunnel) listenUDP() (udpConn, error) {
	var fromUDPConn udpConn
	if t.fromNKN {
		if t.tsClient == nil {
			return nil, ErrUDPNotSupported
		}
		var err error
		fromUDPConn, err = t.tsClient.ListenUDP(t.config.AcceptAddrs)
		if err != nil {
			return nil, err
		}
	} else {
		a, err := net.ResolveUDPAddr("udp", t.from)
		if err != nil {
			return nil, err
		}
		fromUDPConn, err = net.ListenUDP("udp", a)
		if err != nil {
			return nil, err
		}
	}

	return fromUDPConn, nil
}

func (t *Tunnel) udpPipe(fromUDPConn udpConn) error {
	msg := make([]byte, tuna.MaxUDPBufferSize)
	for {
		if t.IsClosed() {
			break
		}

		var err error
		n, fromAddr, err := fromUDPConn.ReadFrom(msg)
		if err != nil {
			log.Println("fromUDPConn.ReadFrom err:", err)
			break
		}

		toUDPConn, newDial, err := t.getToUDPConn(fromAddr)
		if err != nil {
			log.Println("getToUDPConn err:", err)
			continue
		}

		if conn, ok := toUDPConn.(*net.UDPConn); ok {
			_, _, err = conn.WriteMsgUDP(msg[:n], nil, nil)
		} else {
			_, err = toUDPConn.WriteTo(msg[:n], nil)
		}
		if err != nil {
			log.Println("toUDPConn.WriteTo err:", err)
			continue
		}
		t.udpConnCache.Set(fromAddr.String(), toUDPConn, cache.DefaultExpiration)

		if newDial { // New dialed up UDP, start reverse data pipe.
			go func() {
				msg := make([]byte, tuna.MaxUDPBufferSize)
				for {
					if t.IsClosed() {
						break
					}

					n, _, err := toUDPConn.ReadFrom(msg)
					if err != nil {
						log.Println("toUDPConn.ReadFrom err:", err)
						break
					}
					t.udpConnCache.Set(fromAddr.String(), toUDPConn, cache.DefaultExpiration)

					_, err = fromUDPConn.WriteTo(msg[:n], fromAddr)
					if err != nil {
						log.Println("fromUDPConn.WriteTo err:", err)
						break
					}
				}
			}()
		}
	}

	return nil
}
