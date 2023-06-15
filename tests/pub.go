package tests

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	nkn "github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	tunnel "github.com/nknorg/nkn-tunnel"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/vault"
	"github.com/nknorg/tuna"
	"github.com/nknorg/tuna/pb"
	"github.com/nknorg/tuna/types"
	"github.com/nknorg/tuna/util"
)

// status for synchronize go routine
const (
	tunaNodeStarted      = "tuna node is started"
	tunaSessionConnected = "tuna session is connected"
	tunnelServerIsReady  = "tunnel server is ready"
	tunnelClientIsReady  = "tunnel client is ready"
	tcpServerIsReady     = "tcp server is ready"
	udpServerIsReady     = "udp server is ready"
	exit                 = "exit"
	tcpServerExit        = "tcp server exit"
	udpServerExit        = "udp server exit"
	udpClientExit        = "udp client exit"
)

var ch chan string

func waitFor(ch chan string, status string) {
	fmt.Println("Waiting for:", status)
	for {
		str := <-ch
		fmt.Println("Got:", str)
		if strings.Contains(str, status) {
			break
		}
	}
}

func CreateAccountAndWallet(seedHex string) (acc *nkn.Account, wal *nkn.Wallet, err error) {
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		log.Fatal(err)
		return nil, nil, err
	}

	acc, err = nkn.NewAccount(seed)
	if err != nil {
		log.Fatal(err)
		return
	}

	wal, err = nkn.NewWallet(acc, nil)
	if err != nil {
		log.Fatal(err)
	}
	bal, _ := wal.Balance()
	log.Printf("wallet address is %v, balance is %v", wal.Address(), bal)

	return
}

func CreateTunaSessionConfig(numListener int) (config *ts.Config) {
	config = &ts.Config{
		NumTunaListeners: numListener,
		TunaMaxPrice:     "0.01",
	}
	return config
}

func CreateDialConfig(timeout int32) (config *nkn.DialConfig) {
	config = &nkn.DialConfig{DialTimeout: timeout}
	return
}

func CreateClientConfig(retries int32) (config *nkn.ClientConfig) {
	config = &nkn.ClientConfig{ConnectRetries: retries}
	return
}

func CreateMultiClient(account *nkn.Account, id string, numClient int) (mc *nkn.MultiClient, err error) {
	clientConfig := CreateClientConfig(1)
	mc, err = nkn.NewMultiClient(account, id, numClient, false, clientConfig)
	if err != nil {
		log.Fatal(err)
	}

	<-mc.OnConnect.C
	return
}

func CreateTunaSession(account *nkn.Account, wallet *nkn.Wallet, mc *nkn.MultiClient, numListener int) (tunaSess *ts.TunaSessionClient, err error) {
	config := CreateTunaSessionConfig(numListener)
	tunaSess, err = ts.NewTunaSessionClient(account, mc, wallet, config)
	if err != nil {
		log.Fatal(err)
	}
	return
}

var tunaNode *types.Node

func CreateTunnelConfig(udp bool) *tunnel.Config {
	config := &tunnel.Config{
		NumSubClients:     numClients,
		ClientConfig:      CreateClientConfig(3),
		WalletConfig:      &nkn.WalletConfig{},
		DialConfig:        CreateDialConfig(5000),
		TunaSessionConfig: CreateTunaSessionConfig(numClients),
		Verbose:           true,
		UDP:               udp,
		TunaNode:          tunaNode,
	}

	return config
}

func StartTunaNode() *types.Node {
	// Set up tuna
	tunaPubKey, tunaPrivKey, _ := crypto.GenKeyPair()
	tunaSeed := crypto.GetSeedFromPrivateKey(tunaPrivKey)
	go runReverseEntry(tunaSeed)

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

	return n
}

func runReverseEntry(seed []byte) error {
	entryAccount, err := vault.NewAccountWithSeed(seed)
	if err != nil {
		fmt.Println("runReverseEntry vault.NewAccountWithSeed err ", err)
		return err
	}
	seedRPCServerAddr := nkn.NewStringArray(nkn.DefaultSeedRPCServerAddr...)

	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
	}
	entryWallet, err := nkn.NewWallet(&nkn.Account{Account: entryAccount}, walletConfig)
	if err != nil {
		fmt.Println("runReverseEntry nkn.NewWallet err ", err)
		return err
	}
	entryConfig := new(tuna.EntryConfiguration)
	err = util.ReadJSON("config.reverse.entry.json", entryConfig)
	if err != nil {
		fmt.Println("runReverseEntry util.ReadJSON err ", err)
		return err
	}
	err = tuna.StartReverse(entryConfig, entryWallet)
	if err != nil {
		fmt.Println("runReverseEntry tuna.StartReverse err ", err)
		return err
	}

	ch <- tunaNodeStarted
	return nil
}

func StartTunnelListeners(tuna bool) error {
	acc, _, err := CreateAccountAndWallet(seedHex)
	if err != nil {
		return err
	}

	config := CreateTunnelConfig(tuna)

	tunnels, err := tunnel.NewTunnels(acc, listenerId, []string{"nkn"}, []string{toPort}, tuna, config)
	if err != nil {
		return err
	}
	time.Sleep(10 * time.Second) // wait for tuna node is ready
	if tuna {
		for _, t := range tunnels {
			ts := t.TunaSessionClient()
			<-ts.OnConnect()
			ch <- tunaSessionConnected
		}
	}
	ch <- tunnelServerIsReady
	fmt.Printf("tunnel server is ready, toPort is %v\n", toPort)

	for _, t := range tunnels {
		err = t.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

func StartTunnelDialers(tcp, tuna bool) error {
	acc, _, err := CreateAccountAndWallet(seedHex)
	if err != nil {
		return err
	}

	config := CreateTunnelConfig(tuna)
	var from []string
	if tcp {
		from = fromPorts
	} else {
		from = fromUDPPorts
	}

	tunnels, err := tunnel.NewTunnels(acc, dialerId, from, remoteAddrs, tuna, config)
	if err != nil {
		return err
	}

	for _, t := range tunnels {
		go func(t *tunnel.Tunnel) {
			err := t.Start()
			if err != nil {
				fmt.Printf("tunnel.Start err: %v\n", err)
				return
			}
		}(t)
	}

	time.Sleep(5 * time.Second) // Tunnel start time
	ch <- tunnelClientIsReady
	return nil
}
