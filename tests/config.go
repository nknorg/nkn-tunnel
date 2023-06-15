package tests

const (
	bytesToSend = 1024
	bufLen      = 100
	numClients  = 1
	dialerId    = "Alice"
	seedHex     = "e68e046d13dd911594576ba0f4a196e9666790dc492071ad9ea5972c0b940435"

	listenerId = "Bob1"
	toPort     = "127.0.0.1:54321"
)

var fromPorts = []string{"127.0.0.1:12345"}
var fromUDPPorts = []string{"127.0.0.1:22345"}
var remoteAddrs = []string{"Bob1.be285ff9330122cea44487a9618f96603fde6d37d5909ae1c271616772c349fe"}
