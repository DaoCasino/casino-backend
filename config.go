package main

import broker "github.com/DaoCasino/platform-action-monitor-client"

type Config struct {
    Server struct {
        Port     int    `envconfig:"SERVER_PORT" default:"80"`
        LogLevel string `default:"INFO"`
    }
    Broker struct {
        TopicOffsetPath string `envconfig:"OFFSET_PATH"`
        URL             string `envconfig:"BROKER_URL"`
        TopicID         broker.EventType
    }
    BlockChain struct {
        DepositKeyPath    string
        SigniDiceKeyPath  string
        RSAKeyPath        string
        URL               string `envconfig:"BLOCKCHAIN_URL"`
        ChainID           string
        CasinoAccountName string `envconfig:"CASINO_ACCOUNT_NAME"`
    }
}

const (
    defaultConfigPath = "/etc/casino/config.toml"
    configEnvVar      = "CONFIG_PATH"
)
