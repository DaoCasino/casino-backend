package main

import (
    "io/ioutil"
    "strconv"
    "strings"

    "github.com/BurntSushi/toml"
    "github.com/kelseyhightower/envconfig"

    "github.com/rs/zerolog/log"
)


type Config struct {
    Server struct {
        Port int `envconfig:"SERVER_PORT"`
        LogLevel string `envconfig:"LOG_LEVEL"`
    }
    Broker struct {
        TopicOffsetPath string `envconfig:"OFFSET_PATH""`
    }
    BlockChain struct {
        PrivateKeyPath string `envconfig:"PRIVATEKEY_PATH"`
        Url string
        ChainID string
    }
}

func readWIF(filename string) string {
    content, err := ioutil.ReadFile(filename)
    if err != nil {
        log.Panic().Msg(err.Error())
    }
    wif := strings.TrimSpace(strings.TrimSuffix(string(content), "\n"))
    return wif
}


func readConfigFile(cfg *Config) {
    _, err  := toml.DecodeFile("/etc/casino/config.toml", &cfg)
    if err != nil {
        log.Panic().Msg(err.Error())
    }
}

func readEnv(cfg *Config) {
    err := envconfig.Process("", cfg)
    if err != nil {
        log.Panic().Msg(err.Error())
    }
}

func getAddr(port int) string {
    return ":" + strconv.Itoa(port)
}

func main() {
    app := App{}
    cfg := Config{}
    readConfigFile(&cfg)
    readEnv(&cfg)
    log.Info().Msg(cfg.Broker.TopicOffsetPath)
    app.Initialize(
        readWIF(cfg.BlockChain.PrivateKeyPath), cfg.BlockChain.Url, cfg.BlockChain.ChainID,
        cfg.Broker.TopicOffsetPath, cfg.Server.LogLevel)
    app.Run(getAddr(cfg.Server.Port))
}
