package main

import broker "github.com/DaoCasino/platform-action-monitor-client"

type Config struct {
	Server struct {
		Port     int    `default:"80"`
		LogLevel string `default:"INFO"`
	}
	Broker struct {
		TopicOffsetPath      string
		URL                  string
		TopicID              broker.EventType
		ReconnectionAttempts int `default:"3"`
		ReconnectionDelay    int `default:"3"`
		Token                string
	}
	BlockChain struct {
		DepositKey           string
		SigniDiceKey         string
		SigniDiceAccountName string
		RSAKey               string
		URL                  string
		ChainID              string
		PlatformAccountName  string
		PlatformPubKey       string
	}
	HTTP struct {
		RetryAmount int `default:"3"`
		RetryDelay  int `default:"1"`
		Timeout     int `default:"3"`
	}
}

const (
	defaultConfigPath = "/etc/casino/config.dev"
	configEnvVar      = "CONFIG_PATH"
)
