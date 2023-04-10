package tests

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	tunnel "github.com/nknorg/nkn-tunnel"
)

// go test -v -run=TestTCPWriteReadData
func TestTCPWriteReadData(t *testing.T) {
	go func() {
		err := StartTcpServer()
		if err != nil {
			fmt.Printf("StartTcpServer err: %v\n", err)
			return
		}
	}()
	waitFor(ch, tcpServerIsReady)

	tuna := true
	go func() {
		err := StartTunnelListener(toAddrTcp, tuna)
		if err != nil {
			fmt.Printf("StartTunnelListener err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelServerIsReady)

	go func() {
		err := StartTunnelDialer(fromAddrTcp, tuna)
		if err != nil {
			fmt.Printf("StartTunnelDialer err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelClientIsReady)

	go StartTcpDialer()

	waitFor(ch, tcpServerExit)
}

func StartTunnelListener(toAddr string, tuna bool) error {
	acc, _, err := CreateAccountAndWallet(seedHex)
	if err != nil {
		return err
	}

	config := CreateTunnelConfig(true)
	tun, err := tunnel.NewTunnel(acc, listenerId, "nkn", toAddr, tuna, config)
	if err != nil {
		return err
	}
	time.Sleep(10 * time.Second) // wait for tuna node is ready

	ts := tun.TunaSessionClient()
	<-ts.OnConnect()
	ch <- tunaSessionConnected
	ch <- tunnelServerIsReady
	fmt.Printf("tunnel server is ready, toAddr is %v\n", toAddr)

	err = tun.Start()
	if err != nil {
		return err
	}
	return nil
}

func StartTunnelDialer(fromAddr string, tuna bool) error {
	acc, _, err := CreateAccountAndWallet(seedHex)
	if err != nil {
		return err
	}

	config := CreateTunnelConfig(true)
	tun, err := tunnel.NewTunnel(acc, listenerId, fromAddr, remoteAddr, tuna, config)
	if err != nil {
		return err
	}

	ch <- tunnelClientIsReady

	err = tun.Start()
	if err != nil {
		return err
	}
	return nil
}

func StartTcpServer() error {
	listener, err := net.Listen("tcp", toAddrTcp)
	if err != nil {
		return err
	}
	fmt.Printf("tcp server is listening at %v\n", toAddrTcp)
	ch <- tcpServerIsReady

	conn, err := listener.Accept()
	if err != nil {
		return err
	}

	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return err
		}

		fmt.Printf("tcp server read: %v\n", string(b[:n]))
		if strings.Contains(string(b[:n]), tcpDialerExit) {
			break
		}
	}
	ch <- tcpServerExit
	return nil
}

func StartTcpDialer() error {
	conn, err := net.Dial("tcp", fromAddrTcp)
	if err != nil {
		return err
	}

	for i := 0; i < 10; i++ {
		_, err := conn.Write([]byte(fmt.Sprintf("tcp client data %v\n", i)))
		if err != nil {
			return err
		}
	}
	conn.Write([]byte(tcpDialerExit))
	time.Sleep(2 * time.Second) // wait for tcp server get it

	ch <- tcpDialerExit
	return nil
}
