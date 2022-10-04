package relaydaemon

import (
	"encoding/json"
	"os"
	"time"

	relayv1 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv1/relay"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
)

// Config stores the full configuration of the relays, ACLs and other settings
// that influence behaviour of a relay daemon.
type Config struct {
	Network NetworkConfig
	ConnMgr ConnMgrConfig
	RelayV1 RelayV1Config
	RelayV2 RelayV2Config
	ACL     ACLConfig
	Daemon  DaemonConfig
	TLS     TLSConfig
}

// DaemonConfig controls settings for the relay-daemon itself.
type DaemonConfig struct {
	PprofPort int
}

// NetworkConfig controls listen and annouce settings for the libp2p host.
type NetworkConfig struct {
	ListenAddrs   []string
	AnnounceAddrs []string
}

// ConnMgrConfig controls the libp2p connection manager settings.
type ConnMgrConfig struct {
	ConnMgrLo    int
	ConnMgrHi    int
	ConnMgrGrace time.Duration
}

// RelayV1Config controls activation of V1 circuits and resouce configuration
// for them.
type RelayV1Config struct {
	Enabled   bool
	Resources relayv1.Resources
}

// RelayV2Config controls activation of V2 circuits and resouce configuration
// for them.
type RelayV2Config struct {
	Enabled   bool
	Resources relayv2.Resources
}

// ACLConfig provides filtering configuration to allow specific peers or
// subnets to be fronted by relays. In V2, this specifies the peers/subnets
// that are able to make reservations on the relay. In V1, this specifies the
// peers/subnets that can be contacted through the relays.
type ACLConfig struct {
	AllowPeers   []string
	AllowSubnets []string
}

type TLSConfig struct {
	// [][certPath, keyPath]
	KeyPairPaths [][2]string
}

func DefaultConfig() Config {
	return Config{
		Network: NetworkConfig{
			ListenAddrs: []string{
				"/ip4/0.0.0.0/tcp/4011/ws",
            	"/ip6/::/tcp/4011/ws",
			},
		},
		ConnMgr: ConnMgrConfig{
			ConnMgrLo:    96,
			ConnMgrHi:    512,
			ConnMgrGrace: 2 * time.Minute,
		},
		RelayV1: RelayV1Config{
			Enabled:   true,
			Resources: relayv1.DefaultResources(),
		},
		RelayV2: RelayV2Config{
			Enabled:   false,
			Resources: relayv2.DefaultResources(),
		},
		Daemon: DaemonConfig{
			PprofPort: -1,
		},
	}
}

// LoadConfig reads a relay daemon JSON configuration from the given path.
// The configuration is first initialized with DefaultConfig, so all unset
// fields will take defaults from there.
func LoadConfig(cfgPath string) (Config, error) {
	cfg := DefaultConfig()

	if cfgPath != "" {
		cfgFile, err := os.Open(cfgPath)
		if err != nil {
			return Config{}, err
		}
		defer cfgFile.Close()

		decoder := json.NewDecoder(cfgFile)
		err = decoder.Decode(&cfg)
		if err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}
