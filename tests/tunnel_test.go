package tests

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/nknorg/ncp-go"
	"github.com/nknorg/nkn-sdk-go"
	session "github.com/nknorg/nkn-tuna-session"
	tunnel "github.com/nknorg/nkn-tunnel"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/nknorg/tuna"
	"github.com/nknorg/tuna/pb"
	"github.com/nknorg/tuna/tests"
	"github.com/nknorg/tuna/types"
	"github.com/nknorg/tuna/util"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	server := tests.NewServer("0.0.0.0", "12345")
	go server.RunUDPEchoServer()
	go server.RunTCPEchoServer()
	os.Exit(m.Run())
}

func TestTunnel(t *testing.T) {
	_, privKey, _ := crypto.GenKeyPair()
	seed := crypto.GetSeedFromPrivateKey(privKey)

	account, err := nkn.NewAccount(seed)
	if err != nil {
		t.Fatal(err)
	}

	seedRPCServerAddr := nkn.NewStringArray(nkn.DefaultSeedRPCServerAddr...)
	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
	}
	sessionConfig := &ncp.Config{
		MTU: int32(5000),
	}
	clientConfig := &nkn.ClientConfig{
		SessionConfig:     sessionConfig,
		SeedRPCServerAddr: seedRPCServerAddr,
	}

	dialConfig := &nkn.DialConfig{
		DialTimeout:   5000,
		SessionConfig: sessionConfig,
	}

	acceptAddrs := []string{
		"7aafe088fed1a3d2b161437208ce61e26ed1b2d0b83fcef5ec55c273defac1da$",
		"7cafe0ae02789f8eb6b293e46b0ac5cf8f92f73042199c8161e5b5f90b13dcb5$"}

	tsConfig := &session.Config{
		NumTunaListeners:  1,
		TunaMaxPrice:      "0.0",
		TunaMinNanoPayFee: "0.1",
		TunaServiceName:   "reverse",
	}

	// Set up tuna
	tunaPubKey, tunaPrivKey, _ := crypto.GenKeyPair()
	tunaSeed := crypto.GetSeedFromPrivateKey(tunaPrivKey)
	go runReverseEntry(tunaSeed)
	time.Sleep(15 * time.Second)

	n := &types.Node{
		Delay:     0,
		Bandwidth: 0,
		Metadata: &pb.ServiceMetadata{
			Ip:              "127.0.0.1",
			TcpPort:         30020,
			UdpPort:         30021,
			ServiceId:       0,
			Price:           "0.0",
			BeneficiaryAddr: "",
		},
		Address:     hex.EncodeToString(tunaPubKey),
		MetadataRaw: "CgkxMjcuMC4wLjEQxOoBGMXqAToFMC4wMDE=",
	}

	tunnelConfig := &tunnel.Config{
		NumSubClients:     1,
		AcceptAddrs:       nkn.NewStringArray(acceptAddrs...),
		ClientConfig:      clientConfig,
		WalletConfig:      walletConfig,
		DialConfig:        dialConfig,
		TunaSessionConfig: tsConfig,
		Udp:               true,
		Verbose:           true,
		TunaNode:          n,
	}

	tunServer, err := tunnel.NewTunnel(account, "test", "", "127.0.0.1:12345", true, tunnelConfig)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		err := tunServer.Start()
		if err != nil {
			t.Fatal(err)
		}
	}()
	t.Log("Tunnel listen address:", tunServer.FromAddr())

	clientSeed := "444a1e625c4d5b36f8059832a521e88fb219fa31ddf14da8fe23346f59081fb8"
	seed, err = hex.DecodeString(clientSeed)
	if err != nil {
		t.Fatal(err)
	}
	clientAccount, err := nkn.NewAccount(seed)

	tunClient, err := tunnel.NewTunnel(clientAccount, "testclient", "127.0.0.1:54321", tunServer.FromAddr(), true, tunnelConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Client NKN address:", tunClient.Addr().String())

	go func() {
		err := tunClient.Start()
		if err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(10 * time.Second)

	tcpConn, err := net.Dial("tcp", "127.0.0.1:54321")
	err = testTCP(tcpConn)
	if err != nil {
		t.Fatal(err)
	}

	udpConn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 54321,
	})
	err = testUDP(udpConn)
	if err != nil {
		t.Fatal(err)
	}
}

func runReverseEntry(seed []byte) error {
	entryAccount, err := vault.NewAccountWithSeed(seed)
	if err != nil {
		return err
	}
	seedRPCServerAddr := nkn.NewStringArray(nkn.DefaultSeedRPCServerAddr...)

	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
	}
	entryWallet, err := nkn.NewWallet(&nkn.Account{Account: entryAccount}, walletConfig)
	if err != nil {
		return err
	}
	entryConfig := new(tuna.EntryConfiguration)
	err = util.ReadJSON("config.reverse.entry.json", entryConfig)
	if err != nil {
		return err
	}
	err = tuna.StartReverse(entryConfig, entryWallet)
	if err != nil {
		return err
	}
	select {}
}

func testTCP(conn net.Conn) error {
	send := make([]byte, 4096)
	receive := make([]byte, 4096)

	for i := 0; i < 1000; i++ {
		rand.Read(send)
		conn.Write(send)
		io.ReadFull(conn, receive)
		if !bytes.Equal(send, receive) {
			return errors.New("bytes not equal")
		}
	}
	return nil
}

func testUDP(from *net.UDPConn) error {
	count := 1000
	sendList := make([]string, count)
	recvList := make([]string, count)
	sendNum := 0
	recvNum := 0
	var wg sync.WaitGroup
	var e error
	go func() {
		wg.Add(1)
		receive := make([]byte, 1024)
		for i := 0; i < count; i++ {
			_, _, err := from.ReadFromUDP(receive)
			if err != nil {
				e = err
				return
			}
			recvNum++
			recvList = append(recvList, hex.EncodeToString(receive))
		}
		wg.Done()
	}()

	go func() {
		time.Sleep(1 * time.Second)
		send := make([]byte, 1024)
		wg.Add(1)
		for i := 0; i < count; i++ {
			rand.Read(send)
			_, _, err := from.WriteMsgUDP(send, nil, nil)
			if err != nil {
				e = err
				return
			}
			sendNum++
			sendList = append(sendList, hex.EncodeToString(send))
		}
		wg.Done()
	}()

	wg.Wait()
	if sendNum != recvNum {
		return errors.New("package lost")
	}

	for i := 0; i < sendNum; i++ {
		if sendList[i] != recvList[i] {
			return errors.New("data mismatch")
		}
	}

	return e
}
