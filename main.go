package main

import (
	"encoding/hex"
	"flag"
	"github.com/BurntSushi/toml"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/DaoCasino/casino-backend/utils"
	"github.com/eoscanada/eos-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
)

func MakeAppConfig(cfg *Config) (*AppConfig, *eos.KeyBag, error) {
	appCfg := new(AppConfig)
	var err error

	appCfg.Broker.TopicID = cfg.Broker.TopicID

	if f, err := os.Open(cfg.Broker.TopicOffsetPath); err == nil {
		defer f.Close()
		if appCfg.Broker.TopicOffset, err = utils.ReadOffset(f); err != nil {
			return nil, nil, err
		}
	} else {
		// initial start
		appCfg.Broker.TopicOffset = 0
	}

	keyBag := &eos.KeyBag{}

	if err = keyBag.Add(utils.ReadWIF(cfg.BlockChain.DepositKeyPath)); err != nil {
		return nil, nil, err
	}

	if err = keyBag.Add(utils.ReadWIF(cfg.BlockChain.SigniDiceKeyPath)); err != nil {
		return nil, nil, err
	}

	pubKeys, err := keyBag.AvailableKeys()

	if err != nil {
		return nil, nil, err
	}

	appCfg.BlockChain.EosPubKeys = PubKeys{pubKeys[0], pubKeys[1]}
	if appCfg.BlockChain.ChainID, err = hex.DecodeString(cfg.BlockChain.ChainID); err != nil {
		return nil, nil, err
	}
	appCfg.BlockChain.CasinoAccountName = cfg.BlockChain.CasinoAccountName

	if appCfg.BlockChain.RSAKey, err = utils.ReadRsa(cfg.BlockChain.RSAKeyPath); err != nil {
		return nil, nil, err
	}

	return appCfg, keyBag, nil
}

func MakeApp(cfg *Config) (*App, *os.File, error) {
	appConfig, keyBag, err := MakeAppConfig(cfg)
	if err != nil {
		log.Panic().Msgf("Failed to process config, reason: %s", err.Error())
	}

	events := make(chan *broker.EventMessage)

	f, err := os.OpenFile(cfg.Broker.TopicOffsetPath, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, nil, err
	}

	bc := eos.New(cfg.BlockChain.URL)
	bc.SetSigner(keyBag)

	app := NewApp(bc, broker.NewEventListener(cfg.Broker.URL, events), events, f, appConfig)
	return app, f, nil
}

func GetConfig(configPath string) (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", utils.GetConfigPath(configEnvVar, defaultConfigPath), "config file path")
	flag.Parse()

	cfg, err := GetConfig(*configPath)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
	logLevel := cfg.Server.LogLevel
	InitLogger(cfg.Server.LogLevel)

	if strings.ToLower(logLevel) == "debug" {
		broker.EnableDebugLogging()
	}

	app, f, err := MakeApp(cfg)
	if err != nil {
		log.Panic().Msg(err.Error())
	}
	defer f.Close()

	if err := app.Run(utils.GetAddr(cfg.Server.Port)); err != nil {
		log.Panic().Msg(err.Error())
	}
}
