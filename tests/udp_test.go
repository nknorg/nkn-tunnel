package tests

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// go test -v -run=TestUDP
func TestUDP(t *testing.T) {
	ch = make(chan string, 4)
	if tunaNode == nil {
		tunaNode = StartTunaNode()
		waitFor(ch, tunaNodeStarted)
	}

	go func() {
		err := StartUdpServer()
		if err != nil {
			fmt.Printf("StartUdpServer err: %v\n", err)
			return
		}
	}()

	waitFor(ch, udpServerIsReady)
	tuna := true
	go func() {
		err := StartTunnelListeners(tuna)
		if err != nil {
			fmt.Printf("StartTunnelListener err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelServerIsReady)

	go func() {
		err := StartTunnelDialers(false, tuna)
		if err != nil {
			fmt.Printf("StartTunnelDialer err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelClientIsReady)

	err := StartUdpClients()
	if err != nil {
		fmt.Printf("StartUdpClients err: %v\n", err)
		return
	}

	waitFor(ch, udpServerExit)
	close(ch)
}

func StartUdpServer() error {
	a, err := net.ResolveUDPAddr("udp", toPort)
	if err != nil {
		fmt.Println("StartUdpServer ResolveUDPAddr err: ", err)
		return err
	}
	udpServer, err := net.ListenUDP("udp", a)
	if err != nil {
		fmt.Println("StartUdpServer ListenUDP err: ", err)
		return err
	}
	fmt.Printf("udp server is listening at %v\n", toPort)
	ch <- udpServerIsReady

	b := make([]byte, 1024)
	for {
		n, addr, err := udpServer.ReadFromUDP(b)
		if err != nil {
			fmt.Println("StartUdpServer ReadFromUDP err: ", err)
			return err
		}
		fmt.Printf("UDP Server got: %v\n", string(b[:n]))
		time.Sleep(500 * time.Millisecond)
		n, err = udpServer.WriteTo(b[:n], addr)
		if err != nil {
			fmt.Printf("udpServer WriteTo err %v\n", err)
			break
		}
		if strings.Contains(string(b[:n]), exit) {
			fmt.Println("Udp Server got exit, exit now.")
			break
		}
	}

	ch <- udpServerExit
	return nil
}

func StartUdpClients() error {
	var wg sync.WaitGroup
	for i, fromPort := range fromUDPPorts {
		wg.Add(1)
		go func(clientNum int, from string) {
			defer wg.Done()
			fmt.Printf("upd client %v send to port %v\n", clientNum, from)

			a, err := net.ResolveUDPAddr("udp", from)
			if err != nil {
				fmt.Println("StartUdpClient net.ResolveUDPAddr err: ", err)
				return
			}
			udpClient, err := net.DialUDP("udp", nil, a)
			if err != nil {
				fmt.Println("StartUdpClient net.DialUDP err: ", err)
				return
			}

			for j := 0; j < 10; j++ {
				sendData := fmt.Sprintf("udp client %v msg %v", clientNum, j)
				_, _, err := udpClient.WriteMsgUDP([]byte(sendData), nil, nil)
				if err != nil {
					fmt.Printf("StartUdpClient WriteMsgUDP err: %v\n", err)
					return
				}

				recvData := make([]byte, 1024)
				n, _, err := udpClient.ReadFrom(recvData)
				if err != nil {
					fmt.Printf("StartUdpClient.ReadFrom err %v\n", err)
					return
				}

				if string(recvData[:n]) != sendData {
					fmt.Printf("udpClient.ReadFrom is not equal to I sent.\n")
					fmt.Printf("I sent %v,  received: %v\n", sendData, string(recvData[:n]))
				} else {
					fmt.Printf("UDP Client %v got echo: %v\n", clientNum, string(recvData[:n]))
				}
				time.Sleep(100 * time.Millisecond)
			}
			udpClient.WriteMsgUDP([]byte(exit), nil, nil)
			time.Sleep(time.Second)

		}(i, fromPort)

	}

	wg.Wait()
	return nil
}
