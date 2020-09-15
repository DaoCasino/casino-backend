package main

import (
	"encoding/hex"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/eoscanada/eos-go/ecc"

	"github.com/BurntSushi/toml"
	"github.com/DaoCasino/casino-backend/utils"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/eoscanada/eos-go"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

func MakeAppConfig(cfg *Config) (*AppConfig, *eos.KeyBag, error) {
	appCfg := new(AppConfig)
	var err error

	// set broker config
	appCfg.Broker.TopicID = cfg.Broker.TopicID

	if f, err := os.Open(cfg.Broker.TopicOffsetPath); err == nil {
		defer f.Close()
		appCfg.Broker.TopicOffset, err = utils.ReadOffset(f)
		if err != nil {
			if err == io.EOF { // if file empty just set 0
				appCfg.Broker.TopicOffset = 0
			} else {
				return nil, nil, err
			}
		}
	} else {
		// initial start
		appCfg.Broker.TopicOffset = 0
	}

	// set blockchain config
	keyBag := &eos.KeyBag{}
	if err = keyBag.Add(cfg.BlockChain.DepositKey); err != nil {
		return nil, nil, err
	}
	if err = keyBag.Add(cfg.BlockChain.SigniDiceKey); err != nil {
		return nil, nil, err
	}
	pubKeys, err := keyBag.AvailableKeys()
	if err != nil {
		return nil, nil, err
	}
	appCfg.BlockChain.SignerAccountName = eos.AN(cfg.BlockChain.SigniDiceAccountName)
	appCfg.BlockChain.EosPubKeys = PubKeys{pubKeys[0], pubKeys[1]}
	if appCfg.BlockChain.RSAKey, err = utils.ReadRsa(cfg.BlockChain.RSAKey); err != nil {
		return nil, nil, err
	}
	if appCfg.BlockChain.ChainID, err = hex.DecodeString(cfg.BlockChain.ChainID); err != nil {
		return nil, nil, err
	}

	appCfg.BlockChain.PlatformAccountName = eos.AN(cfg.BlockChain.PlatformAccountName)
	if appCfg.BlockChain.PlatformPubKey, err = ecc.NewPublicKey(cfg.BlockChain.PlatformPubKey); err != nil {
		return nil, nil, err
	}

	// set HTTP config
	appCfg.HTTP.RetryDelay = time.Duration(cfg.HTTP.RetryDelay) * time.Second
	appCfg.HTTP.Timeout = time.Duration(cfg.HTTP.Timeout) * time.Second
	appCfg.HTTP.RetryAmount = cfg.HTTP.RetryAmount
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

	brokerClient := broker.NewEventListener(cfg.Broker.URL, events)
	brokerClient.ReconnectionAttempts = cfg.Broker.ReconnectionAttempts
	brokerClient.ReconnectionDelay = time.Duration(cfg.Broker.ReconnectionDelay) * time.Second
	brokerClient.SetToken(cfg.Broker.Token)
	app := NewApp(bc, brokerClient, events, f, appConfig)
	return app, f, nil
}

func GetConfig(configPath string) (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return cfg, nil
}

func main() {
	configPath := flag.String("config", utils.GetConfigPath(configEnvVar, defaultConfigPath),
		"config file path")
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
