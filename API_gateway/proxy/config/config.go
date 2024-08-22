package config

import (
	"os"
	"strings"
)

func LoadDBConfig() (string, string, string, string, string) {
	return os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME")
}

func LoadProxyConfig() (string, string) {
	return os.Getenv("PROXY_HOST"), os.Getenv("PROXY_PORT")
}

type ChainMap struct {
	HTTPEndpoints      map[string][]string
	WebSocketEndpoints map[string][]string
}

// LoadChainMap loads the chain map from environment variables
func LoadChainMap() (map[string][]string, map[string][]string) {
	httpEndpoints := make(map[string][]string)
	wsEndpoints := make(map[string][]string)

	// Define keys for HTTP and WebSocket
	keys := []string{"eth", "fuse", "polygon", "solana", "bsc", "base", "arb", "dfk", "klaytn", "linea", "gnosis", "mantle", "sepolia", "tron", "optimism", "holesky", "thorchain_midgard", "thorchain_api", "thorchain_rpc", "bsc_testnet", "blast", "pyth"}

	// Load endpoints
	for _, key := range keys {
		httpValue := os.Getenv(key + "_HTTP")
		wsValue := os.Getenv(key + "_WS")

		if httpValue != "" {
			httpEndpoints[key] = strings.Split(httpValue, ",")
		}
		if wsValue != "" {
			wsEndpoints[key] = strings.Split(wsValue, ",")
		}
	}

	return httpEndpoints, wsEndpoints
}
