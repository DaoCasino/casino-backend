package main

import (
    "context"
    "crypto/rsa"
    "encoding/hex"
    "encoding/json"
    broker "github.com/DaoCasino/platform-action-monitor-client"
    "github.com/eoscanada/eos-go"
    "github.com/gorilla/mux"
    "github.com/rs/zerolog/log"
    "github.com/zenazn/goji/graceful"
    "os"
    "os/signal"
    "strings"
    "sync"
    "syscall"

    "io/ioutil"
    "net/http"
)

type ResponseWriter = http.ResponseWriter
type Request = http.Request
type JsonResponse = map[string]interface{}

type App struct {
    Router *mux.Router
    Broker struct {
        TopicOffsetPath string
        Url string
        TopicID broker.EventType
    }
    BlockChain struct {
        API *eos.API
        KeyBag eos.KeyBag
        ChainID string
        CasinoAccountName string
        SignidiceKey *rsa.PrivateKey
    }
}

func (app *App) Initialize(wif string, blockChainUrl string, chainID string,
    offsetPath string, brokerURL string, topicID broker.EventType,
    casinoAccountName, level string, signidiceKey *rsa.PrivateKey,
) {
    InitLogger(level)
    if strings.ToLower(level) == "debug" {
        broker.EnableDebugLogging()
    }
    log.Debug().Msg("initializing app")

    app.Router = mux.NewRouter()
    app.BlockChain.API = eos.New(blockChainUrl)
    app.BlockChain.ChainID = chainID
    app.BlockChain.CasinoAccountName = casinoAccountName
    app.BlockChain.SignidiceKey = signidiceKey

    log.Debug().Msg("Reading private key from wif")
    if app.BlockChain.KeyBag.Add(wif) != nil {
        log.Panic().Msg("Malformed private key")
    }
    app.BlockChain.API.SetSigner(&app.BlockChain.KeyBag)
    app.Broker.Url = brokerURL
    app.Broker.TopicOffsetPath = offsetPath
    app.Broker.TopicID = topicID

    app.InitializeRoutes()
}

type BrokerData struct {
    Digest eos.Checksum256 `json:"digest"`
}

func (app *App) processEvent(event *broker.Event) {
    log.Info().Msgf("Processing event %+v", event)
    var data BrokerData
    parseError := json.Unmarshal(event.Data, &data)

    if parseError != nil {
       log.Warn().Msg("Couldnt get digest from event")
       return
    }

    api := app.BlockChain.API
    signature, signError := rsaSign(data.Digest, app.BlockChain.SignidiceKey)

    if signError != nil {
       log.Warn().Msg("Couldnt sign signidice_part_2, reason=" + signError.Error())
       return
    }

    trx, packedTx, err := GetSigndiceTransaction(api, event.Sender, app.BlockChain.CasinoAccountName, event.RequestID, signature)

    if err != nil {
        log.Warn().Msg("couldn't form transaction, reason: " + err.Error())
        return
    }

    log.Debug().Msgf("%+v", trx)

    result, sendError := api.PushTransaction(packedTx)
    if sendError != nil {
        log.Warn().Msg("Failed to send transaction, reason: " + sendError.Error())
        return
    }
    log.Debug().Msg("Successfully signed and sent txn, id: " + result.TransactionID)
}

func (app *App) RunEventListener(parentContext context.Context, wg *sync.WaitGroup) {

    go func(parentContext context.Context) {
        defer wg.Done()
        events := make(chan *broker.EventMessage)

        listener := broker.NewEventListener(app.Broker.Url, events)
        ctx, cancel := context.WithCancel(context.Background())

        if err := listener.ListenAndServe(ctx); err != nil {
            log.Panic().Msg(err.Error())
        }

        defer cancel()

        offsetPath := app.Broker.TopicOffsetPath
        offset := readOffset(offsetPath)
        topicID := app.Broker.TopicID

        log.Debug().Msgf("Subscribing to event type %+v with an offset of %+v", topicID, offset)
        _, err := listener.Subscribe(topicID, offset)

        if err != nil {
            log.Error().Msg("Failed to subscribe")
            return
        }

        for {
           select {
           case <-parentContext.Done():
               log.Debug().Msg("Terminating event listener")
               _, err := listener.Unsubscribe(topicID)
               if err != nil {
                   log.Warn().Msg("Failed to unsubscribe")
               } else {
                   log.Debug().Msg("Event listener successfully terminated")
               }
               return
           case eventMessage, ok := <-events:
               if !ok {
                   log.Info().Msg("Failed to read events")
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
               offset = eventMessage.Events[len(eventMessage.Events) - 1].Offset + 1
               writeOffset(offsetPath, offset)
           }
        }
    }(parentContext)
}

func (app *App) Run(addr string) {
    parentContext, cancel := context.WithCancel(context.Background())
    log.Debug().Msg("starting http server")
    go func() {
        log.Error().Msg(graceful.ListenAndServe(addr, app.Router).Error())
    }()
    log.Debug().Msg("stating event listener")
    var wg sync.WaitGroup
    wg.Add(1)
    app.RunEventListener(parentContext, &wg)

    // Handle signals
    done := make(chan os.Signal, 1)
    signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
    log.Debug().Msg("Waiting for signal")
    <-done
    log.Info().Msg("Terminating service")
    cancel()
    wg.Wait()
    log.Info().Msg("Service successfully terminated")
}

func respondWithError(writer ResponseWriter, code int, message string) {
    respondWithJSON(writer, code, JsonResponse{"error": message})
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
    respondWithJSON(writer, http.StatusOK, JsonResponse{"result":"pong"})
}

func (app *App) SignQuery(writer ResponseWriter, req *Request) {
    log.Info().Msg("Called /sign_transaction")
    rawTransaction, _ := ioutil.ReadAll(req.Body)
    tx := eos.SignedTransaction{}
    err := json.Unmarshal(rawTransaction, &tx)
    if err != nil {
        log.Debug().Msg(err.Error())
        respondWithError(writer, http.StatusBadRequest, "failed to deserialize transaction")
        return
    }

    signedTx, signError := app.SignTransaction(&tx)
    if signError != nil {
        log.Warn().Msg(signError.Error())
        respondWithError(writer, http.StatusInternalServerError, "failed to sign transaction")
        return
    }
    packedTrx, _ := signedTx.Pack(eos.CompressionNone)
    result, sendError := app.BlockChain.API.PushTransaction(packedTrx)
    if sendError != nil {
        log.Debug().Msg(sendError.Error())
        respondWithError(writer, http.StatusBadRequest, "failed to send transaction to the blockchain: " + sendError.Error())
        return
    }

    respondWithJSON(writer, http.StatusOK, JsonResponse{"txid": result.TransactionID})
}

func(app *App) SignTransaction(trx *eos.SignedTransaction) (*eos.SignedTransaction, error) {
    blockchain := app.BlockChain
    publicKeys, _ := blockchain.KeyBag.AvailableKeys()
    chainID, _ := hex.DecodeString(blockchain.ChainID)
    return blockchain.KeyBag.Sign(trx, chainID, publicKeys[0])
}

func (app *App) InitializeRoutes() {
    app.Router.HandleFunc("/ping", app.PingQuery).Methods("GET")
    app.Router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
}

