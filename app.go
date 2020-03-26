package main

import (
    "encoding/json"
    "github.com/eoscanada/eos-go"
    "github.com/gorilla/mux"
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
    }
    BlockChain struct {
        API *eos.API
        KeyBag eos.KeyBag
        ChainID string
    }
}

func (app *App) Initialize(wif string, offsetPath string, blockChainUrl string, chainID string,
    level string) {
    app.Router = mux.NewRouter()
    app.BlockChain.API = eos.New(blockChainUrl)
    app.BlockChain.ChainID = chainID
    app.BlockChain.KeyBag.Add(wif)
    app.Broker.TopicOffsetPath = offsetPath

    InitLogger(level)
    app.InitializeRoutes()
}

func (app *App) Run(addr string) {
    log.Error().Msg(http.ListenAndServe(addr, app.Router).Error())
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
        respondWithError(writer, http.StatusBadRequest, "failed to send transaction to the blockchain " )
        return
    }

    respondWithJSON(writer, http.StatusOK, JsonResponse{"result":"ok"})
}

func(app *App) SignTransaction (trx *eos.SignedTransaction) (*eos.SignedTransaction, error) {
    blockchain := app.BlockChain
    publicKeys, _ := blockchain.KeyBag.AvailableKeys()
    return blockchain.KeyBag.Sign(trx, []byte(blockchain.ChainID), publicKeys[0])
}

func (app *App) InitializeRoutes() {
    app.Router.HandleFunc("/ping", app.PingQuery).Methods("GET")
    app.Router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
}

