package main

import (
    "flag"
    broker "github.com/DaoCasino/platform-action-monitor-client"
)

type Config struct {
    Server struct {
        Port int `envconfig:"SERVER_PORT"`
        LogLevel string `envconfig:"LOG_LEVEL"`
    }
    Broker struct {
        TopicOffsetPath string `envconfig:"OFFSET_PATH"`
        Url string `envconfig:"BROKER_URL"`
        TopicID broker.EventType
    }
    BlockChain struct {
        PrivateKeyPath string `envconfig:"PRIVATEKEY_PATH"`
        Url string
        ChainID string
        CasinoAccountName string
    }
}



func main() {
    configPath := flag.String("config", "/etc/config", "config file path")
    flag.Parse()
    app := App{}
    cfg := Config{}
    readConfigFile(&cfg, *configPath)
    readEnv(&cfg)
    app.Initialize(
        readWIF(cfg.BlockChain.PrivateKeyPath), cfg.BlockChain.Url, cfg.BlockChain.ChainID,
        cfg.Broker.TopicOffsetPath, cfg.Broker.Url, cfg.Broker.TopicID, cfg.BlockChain.CasinoAccountName,
        cfg.Server.LogLevel)
    app.Run(getAddr(cfg.Server.Port))
}
