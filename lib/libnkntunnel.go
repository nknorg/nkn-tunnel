package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/hex"
	"github.com/nknorg/ncp-go"
	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	tunnel "github.com/nknorg/nkn-tunnel"
	"github.com/nknorg/nkngomobile"
	"log"
	"os"
	"strings"
	"sync"
)

var (
	instanceTunnel *tunnel.Tunnel
	tunnelMutex    sync.Mutex
	logMutex       sync.Mutex

	logFilePath string
	logFile     *os.File
	logToFile   bool

	DefaultTunaMaxPrice = "0.01"
	DefaultTunaMinFee   = "0.00001"
	DefaultTunaFeeRatio = 0.1
)

func initLogger() error {
	if logFilePath == "" {
		logToFile = false
		log.SetOutput(os.Stdout)
		return nil
	}

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logToFile = false
		log.SetOutput(os.Stdout)
		log.Println("Failed to open log file, defaulting to stdout:", err)
		return err
	}

	log.SetOutput(logFile)
	logToFile = true
	log.Println("Log initialized, writing to file:", logFilePath)
	return nil
}

func closeLogger() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		log.Println("Closing log file")
		logFile.Close()
		logFile = nil
		logToFile = false
	}
}

//export SetLogFilePath
func SetLogFilePath(path *C.char) {
	logMutex.Lock()
	defer logMutex.Unlock()

	logFilePath = C.GoString(path)
	initLogger()
}

//export StartNknTunnel
func StartNknTunnel(numClients C.int, seedRpcServers *C.char, seedHex *C.char, identifier *C.char,
	from *C.char, to *C.char, udp C.int, useTuna C.int,
	tunaMaxPrice *C.char, tunaMinFee *C.char, tunaFeeRatio C.float,
	tunaDownloadGeoDB C.int, tunaGeoDBPath *C.char, tunaMeasureBandwidth C.int, tunaMeasureStoragePath *C.char, tunaMeasurementBytesDownLink C.int,
	verbose C.int) C.int {
	tunnelMutex.Lock()
	defer tunnelMutex.Unlock()

	if instanceTunnel != nil {
		log.Println("Closing existing tunnel before starting a new one...")
		instanceTunnel.Close()
		instanceTunnel = nil
	}

	numClientsGo := int(numClients)
	seedRpcServersGo := C.GoString(seedRpcServers)
	seedHexGo := C.GoString(seedHex)
	identifierGo := C.GoString(identifier)
	fromGo := C.GoString(from)
	toGo := C.GoString(to)
	udpGo := udp != 0
	useTunaGo := useTuna != 0
	tunaMaxPriceGo := C.GoString(tunaMaxPrice)
	tunaMinFeeGo := C.GoString(tunaMinFee)
	tunaFeeRatioGo := float64(tunaFeeRatio)
	tunaDownloadGeoDBGo := tunaDownloadGeoDB != 0
	tunaGeoDBPathGo := C.GoString(tunaGeoDBPath)
	tunaMeasureBandwidthGo := tunaMeasureBandwidth != 0
	tunaMeasureStoragePathGo := C.GoString(tunaMeasureStoragePath)
	tunaMeasurementBytesDownLinkGo := int32(tunaMeasurementBytesDownLink)
	verboseGo := verbose != 0

	if seedHexGo == "" {
		log.Println("Seed hex cannot be empty")
		return 1
	}

	if tunaMeasurementBytesDownLinkGo == 0 {
		tunaMeasurementBytesDownLinkGo = 256 << 8
	}

	if tunaMaxPriceGo == "" {
		tunaMaxPriceGo = DefaultTunaMaxPrice
	}
	if tunaMinFeeGo == "" {
		tunaMinFeeGo = DefaultTunaMinFee
	}
	if tunaFeeRatioGo == 0 {
		tunaFeeRatioGo = DefaultTunaFeeRatio
	}

	seedRpcServerList := strings.Split(seedRpcServersGo, ",")
	seedRpcServerAddr := nkngomobile.NewStringArray(seedRpcServerList...)

	var seed []byte
	var err error

	seed, err = hex.DecodeString(seedHexGo)
	if err != nil {
		log.Println("Invalid seed hex: ", err)
		return 2
	}
	account, err := nkn.NewAccount(seed)
	if err != nil {
		log.Println("Failed to create account:", err)
		return 3
	}
	clientConfig := &nkn.ClientConfig{
		SeedRPCServerAddr: seedRpcServerAddr,
	}
	walletConfig := &nkn.WalletConfig{
		SeedRPCServerAddr: seedRpcServerAddr,
	}
	sessionConfig := &ncp.Config{
		MTU: int32(0),
	}

	var tsConfig *ts.Config
	if useTunaGo {
		tsConfig = &ts.Config{
			NumTunaListeners:             numClientsGo,
			SessionConfig:                sessionConfig,
			TunaMaxPrice:                 tunaMaxPriceGo,
			TunaMinNanoPayFee:            tunaMinFeeGo,
			TunaNanoPayFeeRatio:          tunaFeeRatioGo,
			TunaDownloadGeoDB:            tunaDownloadGeoDBGo,
			TunaGeoDBPath:                tunaGeoDBPathGo,
			TunaMeasureBandwidth:         tunaMeasureBandwidthGo,
			TunaMeasureStoragePath:       tunaMeasureStoragePathGo,
			TunaMeasurementBytesDownLink: tunaMeasurementBytesDownLinkGo,
		}
	}

	config := &tunnel.Config{
		NumSubClients:     numClientsGo,
		ClientConfig:      clientConfig,
		WalletConfig:      walletConfig,
		TunaSessionConfig: tsConfig,
		UDP:               udpGo,
		Verbose:           verboseGo,
	}
	t, err := tunnel.NewTunnel(account, identifierGo, fromGo, toGo, useTunaGo, config, nil)
	if err != nil {
		log.Println("Failed to create tunnel:", err)
		return 4
	}

	instanceTunnel = t

	go func() {
		if err := t.Start(); err != nil {
			log.Println("Tunnel failed to start:", err)
		}
	}()
	log.Println("Tunnel started successfully")
	return 0
}

//export CloseNknTunnel
func CloseNknTunnel() C.int {
	tunnelMutex.Lock()
	defer tunnelMutex.Unlock()

	if instanceTunnel == nil {
		log.Println("No tunnel to close")
		return -1
	}

	instanceTunnel.Close()
	instanceTunnel = nil
	log.Println("Tunnel closed successfully")
	return 0
}

func main() {
	defer closeLogger()
}
