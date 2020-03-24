package main

import (
    "io/ioutil"
    "strconv"
    "strings"

    "github.com/BurntSushi/toml"
    "github.com/eoscanada/eos-go/ecc"
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
    }
}

func readWIF(filename string) *ecc.PrivateKey {
    content, err := ioutil.ReadFile(filename)
    if err != nil {
        log.Panic().Msg(err.Error())
    }
    wif := strings.TrimSpace(strings.TrimSuffix(string(content), "\n"))
    pk, err := ecc.NewPrivateKey(wif)
    if err != nil {
        log.Panic().Msg(err.Error())
    }
    return pk
}


func readConfigFile(cfg *Config) {
    _, err  := toml.DecodeFile("config.toml", &cfg)
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
    app.Initialize(readWIF(cfg.BlockChain.PrivateKeyPath), cfg.Broker.TopicOffsetPath, cfg.Server.LogLevel)
    app.Run(getAddr(cfg.Server.Port))
}
