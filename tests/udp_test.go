package tests

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// go test -v -run=TestUDP
func TestUDP(t *testing.T) {
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
		err := StartTunnelListener(toAddrUdp, tuna)
		if err != nil {
			fmt.Printf("StartTunnelListener err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelServerIsReady)

	go func() {
		err := StartTunnelDialer(fromAddrUdp, tuna)
		if err != nil {
			fmt.Printf("StartTunnelDialer err: %v\n", err)
			return
		}
	}()

	waitFor(ch, tunnelClientIsReady)

	for i := 0; i < 2; i++ {
		go func(clientNum int) {
			err := StartUdpClient(clientNum)
			if err != nil {
				fmt.Printf("StartTunnelDialer %v err: %v\n", clientNum, err)
				return
			}
		}(i)
	}

	waitFor(ch, udpServerExit)
}

func StartUdpServer() error {
	a, err := net.ResolveUDPAddr("udp", toAddrUdp)
	if err != nil {
		return err
	}
	udpServer, err := net.ListenUDP("udp", a)
	if err != nil {
		return err
	}
	ch <- udpServerIsReady

	b := make([]byte, 1024)
	for {
		n, addr, err := udpServer.ReadFromUDP(b)
		if err != nil {
			return err
		}
		fmt.Printf("UDP Server got: %v\n", string(b[:n]))
		time.Sleep(500 * time.Millisecond)
		udpServer.WriteTo(b[:n], addr)
		if strings.Contains(string(b[:n]), udpClientExit) {
			break
		}
	}

	ch <- udpServerExit
	return nil
}

func StartUdpClient(clientNo int) error {
	a, err := net.ResolveUDPAddr("udp", fromAddrUdp)
	if err != nil {
		return err
	}
	udpClient, err := net.DialUDP("udp", nil, a)
	if err != nil {
		return err
	}

	for j := 0; j < 10; j++ {
		sendData := fmt.Sprintf("udp client %v am at %v", clientNo, j)
		n, _, err := udpClient.WriteMsgUDP([]byte(sendData), nil, nil)
		if err != nil {
			fmt.Printf("StartUdpClient WriteMsgUDP err: %v\n", err)
			return err
		}

		recvData := make([]byte, 1024)
		n, _, err = udpClient.ReadFrom(recvData)
		if err != nil {
			fmt.Printf("StartUdpClient.ReadFrom err %v\n", err)
			return err
		}

		if string(recvData[:n]) != sendData {
			fmt.Printf("udpClient.ReadFrom is not equal to I sent.\n")
			fmt.Printf("I sent %v,  received: %v\n", sendData, string(recvData[:n]))
		}
		time.Sleep(1 * time.Second)
	}
	udpClient.WriteMsgUDP([]byte(udpClientExit), nil, nil)
	time.Sleep(time.Second)
	ch <- udpClientExit
	return nil
}
