package main

import (
	"encoding/hex"
	"flag"
	"log"

	tunnel "github.com/nknorg/nkn-tunnel"
)

func main() {
	numClients := flag.Int("n", 4, "number of clients")
	seedHex := flag.String("s", "", "secret seed")
	identifier := flag.String("i", "", "NKN address identifier")
	from := flag.String("from", "", "from address (\"nkn\" or ip:port)")
	to := flag.String("to", "", "to address (nkn address or ip:port)")

	flag.Parse()

	if len(*from) == 0 {
		log.Fatal("From address is empty")
	}

	if len(*to) == 0 {
		log.Fatal("To address is empty")
	}

	seed, err := hex.DecodeString(*seedHex)
	if err != nil {
		log.Fatal(err)
	}

	t, err := tunnel.NewTunnel(*numClients, seed, *identifier, *from, *to)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(t.Start())
}
