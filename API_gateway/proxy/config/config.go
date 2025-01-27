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

	// Iterate over all environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		key, value := parts[0], parts[1]

		if strings.HasSuffix(key, "_HTTP") {
			chain := strings.TrimSuffix(key, "_HTTP")
			httpEndpoints[chain] = strings.Split(value, ",")
		} else if strings.HasSuffix(key, "_WS") {
			chain := strings.TrimSuffix(key, "_WS")
			wsEndpoints[chain] = strings.Split(value, ",")
		}
	}

	return httpEndpoints, wsEndpoints
}
