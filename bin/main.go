package main

import (
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

	t, err := tunnel.NewTunnel(*numClients, *seedHex, *identifier, *from, *to)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(t.Start())
}
