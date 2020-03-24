// main_test.go

package main

import (
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
	a.Initialize(pk, "/var/log/offsets")
	code := m.Run()
	os.Exit(code)
}

func TestPingQuery(t *testing.T) {
	assert := assert.New(t)

	req, _ := http.NewRequest("GET", "/ping", nil)
	response := executeRequest(req)
	assert.Equal(response.Body.String(), "{\"result\":\"pong\"}", "/ping failed")
}

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	a.Router.ServeHTTP(rr, req)

	return rr
}