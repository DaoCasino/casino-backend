package main

import (
	"fmt"
	"github.com/rs/zerolog"
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
		fmt.Println(err.Error())
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

func getLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func main() {
	app := App{}
	cfg := Config{}
	readConfigFile(&cfg)
	readEnv(&cfg)
	zerolog.SetGlobalLevel(getLevel(cfg.Server.LogLevel))
	app.Initialize(readWIF(cfg.BlockChain.PrivateKeyPath), cfg.Broker.TopicOffsetPath)
	app.Run(getAddr(cfg.Server.Port))
}
