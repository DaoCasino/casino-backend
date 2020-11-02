package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/DaoCasino/casino-backend/metrics"

	"github.com/DaoCasino/casino-backend/utils"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/zenazn/goji/graceful"
	"golang.org/x/sync/errgroup"
)

const (
	GetInfoCacheTTL      = 1   // seconds
	EosInternalErrorCode = 500 // internal error HTTP code
	// see: https://github.com/DaoCasino/DAObet/blob/master/libraries/chain/include/eosio/chain/exceptions.hpp
	EosInternalDuplicateErrorCode = 3040008
	ServiceName                   = "casino"
)

type ResponseWriter = http.ResponseWriter
type Request = http.Request
type JSONResponse = map[string]interface{}

type BrokerConfig struct {
	TopicID     broker.EventType
	TopicOffset uint64
}

type PubKeys struct {
	Deposit    ecc.PublicKey
	SigniDice  ecc.PublicKey
	BonusAdmin ecc.PublicKey
}

type BlockChainConfig struct {
	ChainID             eos.Checksum256
	SignerAccountName   eos.AccountName
	CasinoAccountName   eos.AccountName
	EosPubKeys          PubKeys
	RSAKey              *rsa.PrivateKey
	PlatformAccountName eos.AccountName
	PlatformPubKey      ecc.PublicKey
}

type HTTPConfig struct {
	RetryAmount int
	RetryDelay  time.Duration
	Timeout     time.Duration
}

type BonusConfig struct {
	AdminAccountName eos.AccountName
}

type AppConfig struct {
	Broker     BrokerConfig
	BlockChain BlockChainConfig
	HTTP       HTTPConfig
	Bonus      BonusConfig
}

type App struct {
	bcAPI            *eos.API
	lastGetInfoStamp time.Time
	lastGetInfoLock  sync.Mutex
	lastCachedInfo   *eos.InfoResp
	BrokerClient     EventListener
	OffsetHandler    utils.FileStorage
	EventMessages    chan *broker.EventMessage
	*AppConfig
}

type EventListener interface {
	ListenAndServe(ctx context.Context) error
	Subscribe(eventType broker.EventType, offset uint64) (bool, error)
	Unsubscribe(eventType broker.EventType) (bool, error)
	Run(ctx context.Context)
}

func NewApp(bcAPI *eos.API, brokerClient EventListener, eventMessages chan *broker.EventMessage,
	offsetHandler utils.FileStorage,
	cfg *AppConfig) *App {
	return &App{bcAPI: bcAPI, BrokerClient: brokerClient, OffsetHandler: offsetHandler,
		EventMessages: eventMessages, AppConfig: cfg}
}

func (app *App) getTxOpts() (*eos.TxOptions, error) {
	app.lastGetInfoLock.Lock()
	defer app.lastGetInfoLock.Unlock()

	var info *eos.InfoResp

	if !app.lastGetInfoStamp.IsZero() && time.Now().Add(-GetInfoCacheTTL*time.Second).Before(app.lastGetInfoStamp) {
		info = app.lastCachedInfo
	} else {
		var err error
		info, err = app.bcAPI.GetInfo()
		if err != nil {
			return nil, err
		}
		app.lastGetInfoStamp = time.Now()
		app.lastCachedInfo = info
	}

	return &eos.TxOptions{
		ChainID:     info.ChainID,
		HeadBlockID: info.LastIrreversibleBlockID, // set lib as TAPOS block reference
	}, nil
}

func (app *App) processEvent(event *broker.Event) *string {
	log.Debug().Msgf("Processing event %+v", event)
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		metrics.SigniDiceProcessingTimeMs.Observe(elapsed.Seconds() * 1000)
	}()
	var data struct {
		Digest eos.Checksum256 `json:"digest"`
	}
	parseError := json.Unmarshal(event.Data, &data)
	if parseError != nil {
		log.Error().Msgf("Couldnt get digest from event, "+
			"sessionID: %d, reason: %s", event.RequestID, parseError.Error())
		return nil
	}

	api := app.bcAPI
	signature, signError := utils.RsaSign(data.Digest, app.BlockChain.RSAKey)

	if signError != nil {
		log.Error().Msgf("Couldnt sign signidice_part_2, "+
			"sessionID: %d, reason: %s", event.RequestID, signError.Error())
		return nil
	}

	var txOpts *eos.TxOptions
	err := utils.RetryWithTimeout(func() error {
		var e error
		txOpts, e = app.getTxOpts()
		return e
	}, app.HTTP.RetryAmount, app.HTTP.Timeout, app.HTTP.RetryDelay)
	if err != nil {
		log.Error().Msgf("Failed to get blockchain state, "+
			"sessionID: %d, reason: %s", event.RequestID, err.Error())
		return nil
	}

	packedTrx, err := GetSigndiceTransaction(api, eos.AN(event.Sender), app.BlockChain.SignerAccountName,
		event.RequestID, signature, app.BlockChain.EosPubKeys.SigniDice, txOpts)

	if err != nil {
		log.Error().Msgf("Couldn't form signidice_part_2 trx, "+
			"sessionID: %d, reason: %s", event.RequestID, err.Error())
		return nil
	}

	trxID, err := packedTrx.ID()
	if err != nil {
		log.Warn().Msgf("failed to calc trx ID, reason: %s", err.Error())
		return nil
	}
	trxHexEncoded := trxID.String()
	if sendError := SendPackedTrxWithRetries(app.bcAPI, packedTrx, trxHexEncoded,
		app.HTTP.RetryAmount, app.HTTP.Timeout, app.HTTP.RetryDelay); sendError != nil {
		log.Error().Msgf("Failed to send signidice_part_2 trx, "+
			"sessionID: %d, reason: %s", event.RequestID, sendError.Error())
		return nil
	}
	log.Info().Msgf("Successfully sent signidice_part_2 txn, "+
		"sessionID: %d, trxID: %s", event.RequestID, trxHexEncoded)
	return &trxHexEncoded
}

func (app *App) RunEventProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case eventMessage, ok := <-app.EventMessages:
			if !ok {
				log.Debug().Msg("Shutting down cause event-monitor isn't responding")
				return
			}
			if len(eventMessage.Events) == 0 {
				log.Debug().Msg("Gotta event message with no events")
				break
			}
			log.Debug().Msgf("Processing %+v events", len(eventMessage.Events))
			for _, event := range eventMessage.Events {
				go app.processEvent(event)
			}
			offset := eventMessage.Offset + 1
			if err := utils.WriteOffset(app.OffsetHandler, offset); err != nil {
				log.Error().Msgf("Failed to write offset, reason: %s", err.Error())
			}
		}
	}
}

func (app *App) Run(addr string) error {
	ctx, cancel := context.WithCancel(context.Background())
	errGroup, ctx := errgroup.WithContext(ctx)
	defer cancel()

	// no errGroup because ctx close cannot be handled
	go func() {
		defer cancel()
		log.Debug().Msg("starting http server")
		log.Panic().Msg(graceful.ListenAndServe(addr, app.GetRouter()).Error())
	}()

	errGroup.Go(func() error {
		defer cancel()
		log.Debug().Msg("starting event listener")
		go app.BrokerClient.Run(ctx)
		if _, err := app.BrokerClient.Subscribe(app.Broker.TopicID, app.Broker.TopicOffset); err != nil {
			return err
		}
		log.Debug().Msgf("starting event processor with offset %v", app.Broker.TopicOffset)
		app.RunEventProcessor(ctx)
		return nil
	})

	errGroup.Go(func() error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-ctx.Done():
			return nil
		case <-quit:
			cancel()
		}
		return nil
	})

	return errGroup.Wait()
}

func respondWithError(writer ResponseWriter, code int, message string) {
	respondWithJSON(writer, code, JSONResponse{"error": message})
}

func respondWithJSON(writer ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(code)
	_, err := writer.Write(response)
	if err != nil {
		log.Warn().Msg("Failed to respond to client")
	}
}

func (app *App) PingQuery(writer ResponseWriter, req *Request) {
	respondWithJSON(writer, http.StatusOK, JSONResponse{"result": "pong"})
}

func (app *App) WhoQuery(writer ResponseWriter, req *Request) {
	writer.WriteHeader(http.StatusOK)
	_, err := writer.Write([]byte(ServiceName))
	if err != nil {
		log.Warn().Msg("Failed to respond to client")
	}
}

func (app *App) SignQuery(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /sign_transaction")
	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		metrics.SignTransactionProcessingTimeMs.Observe(elapsed.Seconds() * 1000)
	}()
	rawTransaction, _ := ioutil.ReadAll(req.Body)
	tx := &eos.SignedTransaction{}
	err := json.Unmarshal(rawTransaction, tx)
	if err != nil {
		log.Debug().Msgf("failed to deserialize transaction, reason: %s", err.Error())
		respondWithError(writer, http.StatusBadRequest, "failed to deserialize transaction")
		return
	}
	if err := ValidateDepositTransaction(tx, app.BlockChain.CasinoAccountName, app.BlockChain.PlatformAccountName,
		app.BlockChain.PlatformPubKey,
		app.BlockChain.ChainID); err != nil {
		log.Debug().Msgf("invalid transaction supplied, reason: %s", err.Error())
		respondWithError(writer, http.StatusBadRequest, "invalid transaction supplied")
		return
	}
	signedTx, signError := app.bcAPI.Signer.Sign(tx, app.BlockChain.ChainID, app.BlockChain.EosPubKeys.Deposit)

	if signError != nil {
		log.Warn().Msgf("failed to sign transaction, reason: %s", signError.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to sign transaction")
		return
	}
	log.Debug().Msg(signedTx.String())
	packedTrx, _ := signedTx.Pack(eos.CompressionNone)
	trxID, err := packedTrx.ID()
	if err != nil {
		log.Warn().Msgf("failed to calc trx ID, reason: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to calc trx ID")
		return
	}

	if sendError := SendPackedTrxWithRetries(app.bcAPI, packedTrx, trxID.String(),
		app.HTTP.RetryAmount, app.HTTP.Timeout, app.HTTP.RetryDelay); sendError != nil {
		log.Debug().Msgf("failed to send transaction to the blockchain, reason: %s", sendError.Error())
		respondWithError(writer, http.StatusBadRequest, "failed to send transaction to the blockchain, reason: "+
			sendError.Error())
		return
	}

	respondWithJSON(writer, http.StatusOK, JSONResponse{"txid": trxID.String()})
}

func (app *App) GetBonusPlayers(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /admin/bonus_players")

	playerStats, err := app.getBonusPlayers()
	if err != nil {
		log.Debug().Msgf("failed to get bonus players: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to get bonus players: "+err.Error())
	}

	respondWithJSON(writer, http.StatusOK, playerStats)
}

func (app *App) ConvertBonus(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /admin/convert_bonus")

	convertBonusRequest := struct {
		Player string `json:"player"`
		Force  bool   `json:"force"`
	}{}

	if err := json.NewDecoder(req.Body).Decode(&convertBonusRequest); err != nil {
		log.Debug().Msgf("failed to decode convert bonus request: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to decode convert bonus request: "+err.Error())
	}

	if err := app.convertBonus(convertBonusRequest.Player, convertBonusRequest.Force); err != nil {
		log.Debug().Msgf("failed to convert bonus: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to convert bonus: "+err.Error())
	}

	respondWithJSON(writer, http.StatusOK, nil)
}

func (app *App) AddGameNoBonus(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /admin/add_game_no_bonus")

	addGameNoBonusRequest := struct {
		GameAccount string `json:"game_account"`
	}{}

	if err := json.NewDecoder(req.Body).Decode(&addGameNoBonusRequest); err != nil {
		log.Debug().Msgf("failed to decode add game no bonus request: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to decode add game no bonus request: "+err.Error())
	}

	if err := app.addGameNoBonus(addGameNoBonusRequest.GameAccount); err != nil {
		log.Debug().Msgf("failed to add game no bonus: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to add game no bonus: "+err.Error())
	}

	respondWithJSON(writer, http.StatusOK, nil)
}

func (app *App) RemoveGameNoBonus(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /admin/remove_game_no_bonus")

	rmGameNoBonusRequest := struct {
		GameAccount string `json:"game_account"`
	}{}

	if err := json.NewDecoder(req.Body).Decode(&rmGameNoBonusRequest); err != nil {
		log.Debug().Msgf("failed to decode remove game no bonus request: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to decode remove game no bonus request: "+err.Error())
	}

	if err := app.addGameNoBonus(rmGameNoBonusRequest.GameAccount); err != nil {
		log.Debug().Msgf("failed to remove game no bonus: %s", err.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to remove game no bonus: "+err.Error())
	}

	respondWithJSON(writer, http.StatusOK, nil)
}

func (app *App) GetRouter() *mux.Router {
	var router mux.Router
	router.HandleFunc("/ping", app.PingQuery).Methods("GET")
	router.HandleFunc("/who", app.WhoQuery).Methods("GET")
	router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
	router.Handle("/metrics", metrics.GetHandler())

	adminRouter := router.PathPrefix("/admin").Subrouter()
	adminRouter.HandleFunc("/bonus_players", app.GetBonusPlayers).Methods("GET")
	adminRouter.HandleFunc("/convert_bonus", app.ConvertBonus).Methods("POST")
	adminRouter.HandleFunc("/add_game_no_bonus", app.AddGameNoBonus).Methods("POST")
	adminRouter.HandleFunc("/remove_game_no_bonus", app.RemoveGameNoBonus).Methods("POST")

	return &router
}
