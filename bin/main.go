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
	"github.com/nknorg/nkngomobile"
	"github.com/nknorg/tuna/geo"
)

var (
	Version string
)

func main() {
	numClients := flag.Int("n", 4, "number of clients")
	seedHex := flag.String("s", "", "secret seed")
	identifier := flag.String("i", "", "NKN address identifier")
	from := flag.String("from", "", `listening at address (omitted or "nkn" for listening on nkn address, ip:port for tcp address)`)
	to := flag.String("to", "", "dialing to address (nkn address or ip:port)")
	dialTimeout := flag.Int("t", 0, "dial timeout in milliseconds")
	acceptAddr := flag.String("accept", "", "accept incoming nkn address regex, separated by comma")
	useTuna := flag.Bool("tuna", false, "use tuna instead of nkn client for nkn session")
	tunaCountry := flag.String("country", "", `tuna service node allowed country code, separated by comma, e.g. "US" or "US,CN"`)
	tunaServiceName := flag.String("tsn", "", "tuna reverse service name")
	tunaSubscriptionPrefix := flag.String("tsp", "", "tuna subscription prefix")
	tunaMaxPrice := flag.String("tuna-max-price", "0.01", "tuna max price in unit of NKN/MB")
	tunaMinFee := flag.String("tuna-min-fee", "0.00001", "tuna nanopay minimal txn fee")
	tunaFeeRatio := flag.Float64("tuna-fee-ratio", 0.1, "tuna nanopay txn fee ratio")
	tunaDownloadGeoDB := flag.Bool("tuna-download-geo-db", false, "download tuna geo db to disk")
	tunaGeoDBPath := flag.String("tuna-geo-db-path", ".", "path to store tuna geo db")
	tunaMeasureBandwidth := flag.Bool("tuna-measure-bandwidth", false, "tuna measure bandwidth")
	mtu := flag.Int("mtu", 0, "ncp session mtu")
	rpcAddr := flag.String("rpc", "", "Seed RPC server address, separated by comma")
	udp := flag.Bool("udp", false, "support udp")
	verbose := flag.Bool("v", false, "show logs on dialing/accepting connection")
	version := flag.Bool("version", false, "print version")

	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
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

	var acceptAddrs *nkngomobile.StringArray
	if len(*acceptAddr) > 0 {
		acceptAddrs = nkn.NewStringArrayFromString(strings.ReplaceAll(*acceptAddr, ",", " "))
	}

	var seedRPCServerAddr *nkngomobile.StringArray
	if len(*rpcAddr) > 0 {
		seedRPCServerAddr = nkn.NewStringArrayFromString(strings.ReplaceAll(*rpcAddr, ",", " "))
	}

	sessionConfig := &ncp.Config{
		MTU: int32(*mtu),
	}
	clientConfig := &nkn.ClientConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
		SessionConfig:     sessionConfig,
	}
	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRPCServerAddr,
	}
	dialConfig := &nkn.DialConfig{
		DialTimeout: int32(*dialTimeout),
	}

	var tsConfig *ts.Config
	if *useTuna {
		countries := strings.Split(*tunaCountry, ",")
		locations := make([]geo.Location, len(countries))
		for i := range countries {
			locations[i].CountryCode = strings.TrimSpace(countries[i])
		}

		tsConfig = &ts.Config{
			NumTunaListeners:       *numClients,
			SessionConfig:          sessionConfig,
			TunaIPFilter:           &geo.IPFilter{Allow: locations},
			TunaServiceName:        *tunaServiceName,
			TunaSubscriptionPrefix: *tunaSubscriptionPrefix,
			TunaMaxPrice:           *tunaMaxPrice,
			TunaMinNanoPayFee:      *tunaMinFee,
			TunaNanoPayFeeRatio:    *tunaFeeRatio,
			TunaDownloadGeoDB:      *tunaDownloadGeoDB,
			TunaGeoDBPath:          *tunaGeoDBPath,
			TunaMeasureBandwidth:   *tunaMeasureBandwidth,
		}
	}

	config := &tunnel.Config{
		NumSubClients:     *numClients,
		AcceptAddrs:       acceptAddrs,
		ClientConfig:      clientConfig,
		WalletConfig:      walletConfig,
		DialConfig:        dialConfig,
		TunaSessionConfig: tsConfig,
		Udp:               *udp,
		Verbose:           *verbose,
	}

	t, err := tunnel.NewTunnel(account, *identifier, *from, *to, *useTuna, config)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(t.Start())
}
