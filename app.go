package main

import (
    "context"
    "encoding/json"
    broker "github.com/DaoCasino/platform-action-monitor-client"
    "github.com/eoscanada/eos-go"
    "github.com/gorilla/mux"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"

    "github.com/rs/zerolog/log"
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
    }
}

func readOffset(offsetPath string) uint64 {
    log.Debug().Msg("reading offset from " + offsetPath)
    data, err := ioutil.ReadFile(offsetPath)
    if err != nil {
        log.Panic().Msg("couldn't read offset from file")
    }
    result, parseError := strconv.Atoi(strings.Trim(string(data), "\n"))
    if parseError != nil {
        log.Panic().Msgf("Failed to parse offset from %+v reason=%+v", offsetPath, parseError)
    }
    return uint64(result)
}

func writeOffset(offsetPath string, offset uint64) {
    log.Debug().Msg("writing offset to " + offsetPath)
    err := ioutil.WriteFile(offsetPath, []byte(strconv.Itoa(int(offset))), 0644)
    if err != nil {
        log.Error().Msgf("couldnt save offeset %+v", err.Error())
    }
}

func (app *App) Initialize(wif string, blockChainUrl string, chainID string,
    offsetPath string, brokerURL string, topicID broker.EventType, level string) {
    InitLogger(level)
    log.Debug().Msg("initializing app")

    app.Router = mux.NewRouter()
    app.BlockChain.API = eos.New(blockChainUrl)
    app.BlockChain.ChainID = chainID

    log.Debug().Msg("Reading private key from wif")
    if app.BlockChain.KeyBag.Add(wif) != nil {
        log.Panic().Msg("Malformed private key")
    }

    app.Broker.Url = brokerURL
    app.Broker.TopicOffsetPath = offsetPath
    app.Broker.TopicID = topicID

    app.InitializeRoutes()
}

func processEvent(event *broker.Event) {
    // TODO send signature to the blockchain
    log.Error().Msgf("Unknown event %+v", event)
}

func RunEventListener(parentContext context.Context, brokerURL string, topicID broker.EventType, offsetPath string) {

    events := make(chan *broker.EventMessage)

    listener := broker.NewEventListener(brokerURL, events)
    if err := listener.ListenAndServe(parentContext); err != nil {
        log.Panic().Msg(err.Error())
    }
    offset := readOffset(offsetPath)
    log.Debug().Msgf("Subscribing to event type %+v with an offset of %+v", topicID, offset)
    listener.Subscribe(topicID, offset)

    // start event listener goroutine
    go func(ctx context.Context, events <-chan *broker.EventMessage) {
        for {
            select {
            case <-ctx.Done():
                log.Info().Msg("Terminating event listener")
                listener.Unsubscribe(topicID)
                return
            case eventMessage, ok := <-events:
                if !ok {
                    log.Warn().Msg("Failed to read events")
                    return
                }
                if len(eventMessage.Events) == 0 {
                    log.Debug().Msg("Gotta event message with no events")
                    return
                }
                log.Debug().Msgf("Processing %+v events", len(eventMessage.Events))
                for _, event := range eventMessage.Events {
                    processEvent(event)
                }
                offset = eventMessage.Events[len(eventMessage.Events) - 1].Offset
                writeOffset(offsetPath, offset)
            }
        }
    }(parentContext, events)
}

func (app *App) Run(addr string) {
    parentContext, cancel := context.WithCancel(context.Background())
    log.Debug().Msg("starting http server")
    go func() {
        log.Error().Msg(http.ListenAndServe(addr, app.Router).Error())
    }()
    log.Debug().Msg("stating event listener")
    RunEventListener(parentContext, app.Broker.Url, app.Broker.TopicID, app.Broker.TopicOffsetPath)

    // Handle signals
    done := make(chan os.Signal, 1)
    signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
    log.Debug().Msg("Waiting for signal")
    <-done
    log.Info().Msg("Terminating service")
    cancel()
}

func respondWithError(writer ResponseWriter, code int, message string) {
    respondWithJSON(writer, code, JsonResponse{"error": message})
}

func respondWithJSON(writer ResponseWriter, code int, payload interface{}) {
    response, _ := json.Marshal(payload)
    writer.Header().Set("Content-Type", "application/json")
    writer.WriteHeader(code)
    writer.Write(response)
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
    _, sendError := app.BlockChain.API.PushTransaction(packedTrx)
    if sendError != nil {
        log.Debug().Msg(sendError.Error())
        respondWithError(writer, http.StatusBadRequest, "failed to send transaction to the blockchain: " + sendError.Error())
        return
    }

    respondWithJSON(writer, http.StatusOK, JsonResponse{"result":"ok"})
}

func(app *App) SignTransaction(trx *eos.SignedTransaction) (*eos.SignedTransaction, error) {
    blockchain := app.BlockChain
    publicKeys, _ := blockchain.KeyBag.AvailableKeys()
    return blockchain.KeyBag.Sign(trx, []byte(blockchain.ChainID), publicKeys[0])
}

func (app *App) InitializeRoutes() {
    app.Router.HandleFunc("/ping", app.PingQuery).Methods("GET")
    app.Router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
}

