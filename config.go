package tunnel

import (
	"github.com/imdario/mergo"
	"github.com/nknorg/nkn-sdk-go"
	ts "github.com/nknorg/nkn-tuna-session"
	"github.com/nknorg/nkngomobile"
)

type Config struct {
	NumSubClients     int
	OriginalClient    bool
	AcceptAddrs       *nkngomobile.StringArray
	ClientConfig      *nkn.ClientConfig
	WalletConfig      *nkn.WalletConfig
	DialConfig        *nkn.DialConfig
	TunaSessionConfig *ts.Config
	Udp               bool
	Verbose           bool
}

var defaultConfig = Config{
	NumSubClients:     4,
	OriginalClient:    false,
	AcceptAddrs:       nil,
	ClientConfig:      nil,
	WalletConfig:      nil,
	DialConfig:        nil,
	TunaSessionConfig: nil,
	Udp:               false,
	Verbose:           false,
}

func DefaultConfig() *Config {
	conf := defaultConfig
	return &conf
}

func MergedConfig(conf *Config) (*Config, error) {
	merged := DefaultConfig()
	if conf != nil {
		err := mergo.Merge(merged, conf, mergo.WithOverride)
		if err != nil {
			return nil, err
		}
	}
	return merged, nil
}
