package main

import (
	"encoding/json"
	"github.com/rs/zerolog"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eoscanada/eos-go/ecc"
	"github.com/gorilla/mux"
	"fmt"
	"github.com/rs/zerolog/log"
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

func InitLogger(level string) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	output.FormatLevel = func(i interface{}) string {
		const (
			colorBlack = iota + 30
			colorRed
			colorGreen
			colorYellow
			colorBlue
			colorMagenta
			colorCyan
			colorWhite

			colorBold     = 1
			colorDarkGray = 90
		)
		var colorMap = map[string]int {
			"debug": colorYellow,
			"info": colorGreen,
			"warn": colorBlue,
			"error": colorRed,
		}
		colorize := func(s string, c int) string {
			return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
		}
		level, _ := i.(string)
		return colorize(strings.ToUpper(level), colorMap[level])

	}
	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	output.FormatFieldValue = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}
	log.Logger = log.Output(output)
	zerolog.SetGlobalLevel(getLevel(level))
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}
}
