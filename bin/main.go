package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/nknorg/ncp-go"
	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	tunnel "github.com/nknorg/nkn-tunnel"
	"github.com/nknorg/tuna"
)

var (
	Version string
)

func main() {
	numClients := flag.Int("n", 4, "number of clients")
	seedHex := flag.String("s", "", "secret seed")
	identifier := flag.String("i", "", "NKN address identifier")
	from := flag.String("from", "", "from address (\"nkn\" or ip:port)")
	to := flag.String("to", "", "to address (nkn address or ip:port)")
	acceptAddr := flag.String("accept", "", "accept incoming nkn address regex, separated by comma")
	useTuna := flag.Bool("tuna", false, "use tuna instead of nkn client for nkn session")
	tunaCountry := flag.String("country", "", `tuna service node allowed country code, separated by comma, e.g. "US" or "US,CN"`)
	tunaServiceName := flag.String("tsn", "", "tuna reverse service name")
	tunaSubscriptionPrefix := flag.String("tsp", "", "tuna subscription prefix")
	tunaMaxPrice := flag.String("tuna-max-price", "0.01", "tuna max price in unit of NKN/MB")
	mtu := flag.Int("mtu", 0, "ncp session mtu")
	verbose := flag.Bool("v", false, "show logs on dialing/accepting connection")
	version := flag.Bool("version", false, "print version")

	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
	}

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

	account, err := nkn.NewAccount(seed)
	if err != nil {
		log.Fatal(err)
	}

	var acceptAddrs *nkn.StringArray
	if len(*acceptAddr) > 0 {
		acceptAddrs = nkn.NewStringArrayFromString(strings.ReplaceAll(*acceptAddr, ",", " "))
	}

	sessionConfig := &ncp.Config{MTU: int32(*mtu)}
	clientConfig := &nkn.ClientConfig{SessionConfig: sessionConfig}

	var tsConfig *ts.Config
	if *useTuna {
		countries := strings.Split(*tunaCountry, ",")
		locations := make([]tuna.Location, len(countries))
		for i := range countries {
			locations[i].CountryCode = strings.TrimSpace(countries[i])
		}

		tsConfig = &ts.Config{
			NumTunaListeners:       *numClients,
			SessionConfig:          sessionConfig,
			TunaIPFilter:           &tuna.IPFilter{Allow: locations},
			TunaServiceName:        *tunaServiceName,
			TunaSubscriptionPrefix: *tunaSubscriptionPrefix,
			TunaMaxPrice:           *tunaMaxPrice,
		}
	}

	config := &tunnel.Config{
		NumSubClients:     *numClients,
		AcceptAddrs:       acceptAddrs,
		ClientConfig:      clientConfig,
		TunaSessionConfig: tsConfig,
		Verbose:           *verbose,
	}

	t, err := tunnel.NewTunnel(account, *identifier, *from, *to, *useTuna, config)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(t.Start())
}
