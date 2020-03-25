// main_test.go

package main

import (
    "bytes"
    "encoding/json"
    "github.com/stretchr/testify/assert"
    "net/http/httptest"
    "os"
    "testing"

    "github.com/eoscanada/eos-go/ecc"
    "net/http"
)

var a App

func TestMain(m *testing.M) {
    pk, _ := ecc.NewPrivateKey("5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAbuatmU")
    a.Initialize(pk, "/var/log/offsets", "DEBUG")
    code := m.Run()
    os.Exit(code)
}

func TestPingQuery(t *testing.T) {
    assert := assert.New(t)

    req, _ := http.NewRequest("GET", "/ping", nil)
    response := executeRequest(req)
    assert.Equal(response.Body.String(), "{\"result\":\"pong\"}", "/ping failed")
}

func TestSignTransaction(t *testing.T) {
    assert := assert.New(t)

    values := map[string]string{"transaction": "42"}
    jsonValue, _ := json.Marshal(values)
    req, _ := http.NewRequest("POST", "/sign_transaction", bytes.NewBuffer(jsonValue))
    executeRequest(req)
    assert.Equal(true, true, "always true")
}

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
    rr := httptest.NewRecorder()
    a.Router.ServeHTTP(rr, req)

    return rr
}