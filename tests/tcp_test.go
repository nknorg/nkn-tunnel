package tests

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// go test -v -run=TestTCP
func TestTCP(t *testing.T) {
	ch = make(chan string, 4)
	if tunaNode == nil {
		tunaNode = StartTunaNode()
		waitFor(ch, tunaNodeStarted)
	}

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
		err := StartTunnelListeners(tuna)
		if err != nil {
			fmt.Printf("StartTunnelListeners err: %v\n", err)
			os.Exit(-1)
		}
	}()

	waitFor(ch, tunnelServerIsReady)

	go func() {
		err := StartTunnelDialers(true, tuna)
		if err != nil {
			fmt.Printf("StartTunnelDialer err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelClientIsReady)

	go StartTcpDialers()

	waitFor(ch, tcpServerExit)
	close(ch)
}

func StartTcpServer() error {
	listener, err := net.Listen("tcp", toPort)
	if err != nil {
		return err
	}
	fmt.Printf("StartTcpServer is listening at %v\n", toPort)
	ch <- tcpServerIsReady

	conn, err := listener.Accept()
	if err != nil {
		return err
	}

	b := make([]byte, 4096)
	for {
		n, err := conn.Read(b)
		if err != nil {
			fmt.Printf("StartTcpServer conn.Read err %v\n", err)
			return err
		}

		fmt.Printf("TCP Server got: %v\n", string(b[:n]))
		if strings.Contains(string(b[:n]), exit) {
			break
		}
		// echo
		_, err = conn.Write(b[:n])
		if err != nil {
			fmt.Printf("StartTcpServer conn.Write err %v\n", err)
			return err
		}
	}
	ch <- tcpServerExit
	return nil
}

func StartTcpDialers() error {
	var wg sync.WaitGroup
	for i, fromPort := range fromPorts {
		wg.Add(1)
		go func(clientNum int, from string) {
			defer wg.Done()
			conn, err := net.Dial("tcp", from)
			if err != nil {
				fmt.Printf("StartTcpDialers net.Dial to %v err %v\n", from, err)
				return
			}

			for i := 0; i < 10; i++ {
				msg := fmt.Sprintf("tcp client %v data %v", clientNum, i)
				_, err := conn.Write([]byte(msg))
				if err != nil {
					fmt.Printf("StartTcpDialers conn.Write to %v err %v\n", from, err)
					return
				}
				b := make([]byte, 1024)
				n, err := conn.Read(b)
				if err != nil {
					fmt.Printf("StartTcpDialers conn.Read to %v err %v\n", from, err)
					return
				}
				if string(b[:n]) != msg {
					fmt.Printf("StartTcpDialers get echo %v, it should be %v\n", string(b[:n]), msg)
					return
				}
				fmt.Printf("TCP Client %v got echo: %v\n", clientNum, string(b[:n]))
			}
			conn.Write([]byte(exit))
			time.Sleep(2 * time.Second) // wait for tcp server get it
		}(i, fromPort)
	}
	wg.Wait()
	return nil
}
