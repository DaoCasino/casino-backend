// main_test.go

package main

import (
    "bytes"
    "encoding/json"
    "github.com/eoscanada/eos-go"
    "github.com/rs/zerolog/log"
    "github.com/stretchr/testify/assert"
    "net/http/httptest"
    "os"
    "testing"

    "github.com/eoscanada/eos-go/ecc"
    "net/http"
)

var a App
const privateKey string = "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAbuatmU"
const chainID string = "cda75f235aef76ad91ef0503421514d80d8dbb584cd07178022f0bc7deb964ff"

func TestMain(m *testing.M) {
    _, err := ecc.NewPrivateKey(privateKey)
    if err != nil {
        panic(err)
    }
    a.Initialize(privateKey, "/var/log/offsets", "https://api.daobet.org",
        chainID, "DEBUG")
    code := m.Run()
    os.Exit(code)
}

func TestPingQuery(t *testing.T) {
    assert := assert.New(t)

    req, _ := http.NewRequest("GET", "/ping", nil)
    response := executeRequest(req)
    assert.Equal(response.Body.String(), "{\"result\":\"pong\"}", "/ping failed")
}

func TestSignTransactionNormal(t *testing.T) {
    assert := assert.New(t)

    rawTransaction := []byte(`
{
  "expiration": "2020-03-25T17:41:38",
  "ref_block_num": 33633,
  "ref_block_prefix": 1346981524,
  "max_net_usage_words": 0,
  "max_cpu_usage_ms": 0,
  "delay_sec": 0,
  "context_free_actions": [],
  "actions": [{
      "account": "eosio.token",
      "name": "transfer",
      "authorization": [{
          "actor": "lordofdao",
          "permission": "active"
        }
      ],
      "data": "0000a0262d9a2e8d00a8498ba64b23301027000000000000044245540000000000"
    }
  ],
  "transaction_extensions": [],
  "signatures": [
    "SIG_K1_KZGbvWTgBGeidB1NUVjx3SFubLgCPeDrZztau9AWgUiNEknmT9ajNSEXoKpEbVtx4XuwLebxPWz6hDzUgYbEBxed2SkKGi"
  ],
  "context_free_data": []
}`)
   signedRawTx := []byte(`{"expiration":"2020-03-25T17:41:38","ref_block_num":33633,"ref_block_prefix":1346981524,"max_net_usage_words":0,"max_cpu_usage_ms":0,"delay_sec":0,"context_free_actions":[],"actions":[{"account":"eosio.token","name":"transfer","authorization":[{"actor":"lordofdao","permission":"active"}],"data":"0000a0262d9a2e8d00a8498ba64b23301027000000000000044245540000000000"}],"transaction_extensions":[],"signatures":["SIG_K1_KZGbvWTgBGeidB1NUVjx3SFubLgCPeDrZztau9AWgUiNEknmT9ajNSEXoKpEbVtx4XuwLebxPWz6hDzUgYbEBxed2SkKGi","SIG_K1_KVKV98c5Q7cCGqbSSHYsYYo473TeaibkDoLb5V26BHeioY623wAmNLgo9L86nqcy7gKLE8u9dnDnBLR5UVcL65wwaRF34H"],"context_free_data":[]}`)
   origTx := eos.SignedTransaction{}
   err := json.Unmarshal(rawTransaction, &origTx)
   if err != nil {
      log.Info().Msg(err.Error())
      return
   }

    result, signError := a.SignTransaction(&origTx)
    assert.Nil(signError, "failed to sign transaction")
    byteString, _ := json.Marshal(result)
    assert.Equal(signedRawTx, byteString)
}

func TestSignTransactionBadRequest(t *testing.T) {
    assert := assert.New(t)

    // added sender field
    rawTransaction := []byte(`
{
  "sender": "iamthebest"
  "expiration": "2020-03-25T17:41:38",
  "ref_block_num": 33633,
  "ref_block_prefix": 1346981524,
  "max_net_usage_words": 0,
  "max_cpu_usage_ms": 0,
  "delay_sec": 0,
  "context_free_actions": [],
  "actions": [{
      "account": "eosio.token",
      "name": "transfer",
      "authorization": [{
          "actor": "lordofdao",
          "permission": "active"
        }
      ],
      "data": "0000a0262d9a2e8d00a8498ba64b23301027000000000000044245540000000000"
    }
  ],
  "transaction_extensions": [],
  "signatures": [
    "SIG_K1_KZGbvWTgBGeidB1NUVjx3SFubLgCPeDrZztau9AWgUiNEknmT9ajNSEXoKpEbVtx4XuwLebxPWz6hDzUgYbEBxed2SkKGi"
  ],
  "context_free_data": []
}`)
    req, _ := http.NewRequest("POST", "/sign_transaction", bytes.NewBuffer(rawTransaction))
    response := executeRequest(req)
    assert.Equal(response.Body.String(), `{"error":"failed to deserialize transaction"}`)

}

func executeRequest(req *http.Request) *httptest.ResponseRecorder {
    rr := httptest.NewRecorder()
    a.Router.ServeHTTP(rr, req)

    return rr
}