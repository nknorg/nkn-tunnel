package main

import (
	"encoding/hex"
	"flag"
	"io"
	"log"
	"net"
	"strings"

	nknsdk "github.com/nknorg/nkn-sdk-go"
	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/vault"
)

func main() {
	numClients := flag.Int("n", 4, "number of clients")
	seedHex := flag.String("s", "", "secret seed")
	identifier := flag.String("i", "", "NKN address identifier")
	from := flag.String("from", "", "from address (\"nkn\" or ip:port)")
	to := flag.String("to", "", "to address (nkn address or ip:port)")

	flag.Parse()

	fromNKN := strings.ToLower(*from) == "nkn"
	toNKN := !strings.Contains(*to, ":")
	var m *nknsdk.MultiClient
	var err error

	if fromNKN || toNKN {
		var account *vault.Account
		var err error
		if len(*seedHex) > 0 {
			seed, err := common.HexStringToBytes(*seedHex)
			if err != nil {
				log.Fatal(err)
			}
			account, err = vault.NewAccountWithPrivatekey(crypto.GetPrivateKeyFromSeed(seed))
			if err != nil {
				log.Fatal(err)
			}
		} else {
			account, err = vault.NewAccount()
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Println("Seed:", hex.EncodeToString(account.PrivateKey[:32]))

		m, err = nknsdk.NewMultiClient(account, *identifier, *numClients, false, nknsdk.ClientConfig{ConnectRetries: 1})
		if err != nil {
			log.Fatal(err)
		}
	}

	var listener net.Listener

	if fromNKN {
		listener = m
		*from = m.Addr().String()
	} else {
		listener, err = net.Listen("tcp", *from)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Println("Listening at", *from)

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		var s net.Conn
		if toNKN {
			s, err = m.Dial(*to)
		} else {
			s, err = net.Dial("tcp", *to)
		}
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			io.Copy(c, s)
		}()
		go func() {
			io.Copy(s, c)
		}()
	}
}
