package main

import (
    "encoding/json"
    "github.com/eoscanada/eos-go/ecc"
    "github.com/gorilla/mux"
    "github.com/rs/zerolog/log"
    "net/http"
)

type ResponseWriter = http.ResponseWriter
type Request = http.Request
type JsonResponse = map[string]interface{}

type App struct {
    Router *mux.Router
    PrivateKey *ecc.PrivateKey
    TopicOffsetPath string
}

func (app *App) Initialize(pk *ecc.PrivateKey, offsetPath string, level string) {
    app.Router = mux.NewRouter()
    app.PrivateKey = pk
    app.TopicOffsetPath = offsetPath

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
    // TODO
}

func (app *App) InitializeRoutes() {
    app.Router.HandleFunc("/ping", app.PingQuery).Methods("GET")
    app.Router.HandleFunc("/sign_transaction", app.SignQuery).Methods("POST")
}

