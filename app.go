package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/zenazn/goji/graceful"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"io/ioutil"
	"net/http"
)

type ResponseWriter = http.ResponseWriter
type Request = http.Request
type JSONResponse = map[string]interface{}

type Broker struct {
	TopicID     broker.EventType
	TopicOffset uint64
}

type PubKeys struct {
	Deposit   ecc.PublicKey
	SigniDice ecc.PublicKey
}

type BlockChain struct {
	ChainID           eos.Checksum256
	CasinoAccountName string
	EosPubKeys        PubKeys
	RSAKey            *rsa.PrivateKey
}

type AppConfig struct {
	Broker     Broker
	BlockChain BlockChain
}

type App struct {
	bcAPI         *eos.API
	BrokerClient  *broker.EventListener
	OffsetHandler io.Writer
	EventMessages <-chan *broker.EventMessage
	*AppConfig
}

func NewApp(bcAPI *eos.API, brokerClient *broker.EventListener, eventMessages <-chan *broker.EventMessage,
	offsetHandler io.Writer,
	cfg *AppConfig) *App {
	return &App{bcAPI: bcAPI, BrokerClient: brokerClient, OffsetHandler: offsetHandler,
		EventMessages: eventMessages, AppConfig: cfg}
}

func (app *App) processEvent(event *broker.Event) {
	log.Debug().Msgf("Processing event %+v", event)
	var data struct {
		Digest eos.Checksum256 `json:"digest"`
	}
	parseError := json.Unmarshal(event.Data, &data)

	if parseError != nil {
		log.Error().Msgf("Couldnt get digest from event, reason: %s", parseError.Error())
		return
	}

	api := app.bcAPI
	signature, signError := rsaSign([]byte(data.Digest), app.BlockChain.RSAKey)

	if signError != nil {
		log.Error().Msgf("Couldnt sign signidice_part_2, reason: %s", signError.Error())
		return
	}

	trx, packedTx, err := GetSigndiceTransaction(api, event.Sender, app.BlockChain.CasinoAccountName, event.RequestID,
		signature)

	if err != nil {
		log.Error().Msgf("couldn't form transaction, reason: %s", err.Error())
		return
	}

	log.Debug().Msgf("%+v", trx)

	result, sendError := api.PushTransaction(packedTx)
	if sendError != nil {
		log.Error().Msg("Failed to send transaction, reason: " + sendError.Error())
		return
	}
	log.Debug().Msg("Successfully signed and sent txn, id: " + result.TransactionID)
}

func (app *App) RunEventProcessor(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case eventMessage, ok := <-app.EventMessages:
			if !ok {
				log.Debug().Msg("Failed to read events")
				break
			}
			if len(eventMessage.Events) == 0 {
				log.Debug().Msg("Gotta event message with no events")
				break
			}
			log.Debug().Msgf("Processing %+v events", len(eventMessage.Events))
			for _, event := range eventMessage.Events {
				go app.processEvent(event)
			}
			offset := eventMessage.Events[len(eventMessage.Events)-1].Offset + 1
			if err := writeOffset(app.OffsetHandler, offset); err != nil {
				log.Error().Msgf("Failed to write offset, reason: %s", err.Error())
			}
		}
	}
}

func (app *App) Run(addr string) error {
	go func() {
		log.Debug().Msg("starting http server")
		log.Panic().Msg(graceful.ListenAndServe(addr, app.GetRouter()).Error())
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		log.Info().Msg("Terminating service")
		cancel()
		wg.Wait()
		log.Info().Msg("Service successfully terminated")
	}()

	log.Debug().Msg("starting event listener")

	if err := app.BrokerClient.ListenAndServe(ctx); err != nil {
		return err
	}

	offset := app.Broker.TopicOffset
	if _, err := app.BrokerClient.Subscribe(app.Broker.TopicID, offset); err != nil {
		return err
	}

	wg.Add(1)

	go func() {
		log.Debug().Msgf("starting event processor with offset %v", offset)
		app.RunEventProcessor(ctx, &wg)
	}()

	// Handle signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	log.Info().Msg("Service successfully started")
	<-done
	return nil
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
	log.Info().Msg("Called /ping")
	respondWithJSON(writer, http.StatusOK, JSONResponse{"result": "pong"})
}

func (app *App) SignQuery(writer ResponseWriter, req *Request) {
	log.Info().Msg("Called /sign_transaction")
	rawTransaction, _ := ioutil.ReadAll(req.Body)
	tx := &eos.SignedTransaction{}
	err := json.Unmarshal(rawTransaction, tx)
	if err != nil {
		log.Debug().Msg(err.Error())
		respondWithError(writer, http.StatusBadRequest, "failed to deserialize transaction")
		return
	}

	// TODO get deposit key from config
	signedTx, signError := app.bcAPI.Signer.Sign(tx, app.BlockChain.ChainID)

	if signError != nil {
		log.Warn().Msg(signError.Error())
		respondWithError(writer, http.StatusInternalServerError, "failed to sign transaction")
		return
	}
	log.Debug().Msg(signedTx.String())
	packedTrx, _ := signedTx.Pack(eos.CompressionNone)
	result, sendError := app.bcAPI.PushTransaction(packedTrx)
	if sendError != nil {
		log.Debug().Msg(sendError.Error())
		respondWithError(writer, http.StatusBadRequest, "failed to send transaction to the blockchain: "+sendError.Error())
		return
	}

	respondWithJSON(writer, http.StatusOK, JSONResponse{"txid": result.TransactionID})
}

func (app *App) GetRouter() *mux.Router {
	var router mux.Router
	router.HandleFunc("/ping", app.PingQuery).Methods("GET")
	router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
	return &router
}
