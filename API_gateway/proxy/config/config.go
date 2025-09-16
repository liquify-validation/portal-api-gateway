package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadDBConfig() (string, string, string, string, string) {
	return os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME")
}

func LoadProxyConfig() (string, string) {
	return os.Getenv("PROXY_HOST"), os.Getenv("PROXY_PORT")
}

type FileConfig struct {
	Chains map[string]Chain `yaml:"chains"`
}

type ChainMap struct {
	HTTPEndpoints      map[string][]string
	WebSocketEndpoints map[string][]string
}

type Chain struct {
	Type string     `yaml:"type"`
	HTTP []Endpoint `yaml:"http"`
	WS   []Endpoint `yaml:"ws"`
}

type Endpoint struct {
	URL string `yaml:"url"`
}

// LoadChainMap now returns three maps:
// - httpEndpoints[chain] = []httpURLs
// - wsEndpoints[chain]   = []wsURLs
// - chainTypes[chain]    = type string
func LoadChainMap() (map[string][]string, map[string][]string, map[string]string) {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	fc, err := loadFileConfig(cfgPath)
	if err != nil {
		panic(fmt.Errorf("failed to load chain config: %w", err))
	}

	httpEndpoints := make(map[string][]string, len(fc.Chains))
	wsEndpoints := make(map[string][]string, len(fc.Chains))
	chainTypes := make(map[string]string, len(fc.Chains))

	for chainName, chain := range fc.Chains {
		chainTypes[chainName] = chain.Type

		for _, ep := range chain.HTTP {
			if ep.URL != "" {
				httpEndpoints[chainName] = append(httpEndpoints[chainName], ep.URL)
			}
		}
		for _, ep := range chain.WS {
			if ep.URL != "" {
				wsEndpoints[chainName] = append(wsEndpoints[chainName], ep.URL)
			}
		}
	}

	return httpEndpoints, wsEndpoints, chainTypes
}

func loadFileConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var fc FileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	if len(fc.Chains) == 0 {
		return nil, errors.New("no chains defined in config")
	}
	return &fc, nil
}
