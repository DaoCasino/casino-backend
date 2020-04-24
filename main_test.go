// main_test.go

package main

import (
    "bytes"
    broker "github.com/DaoCasino/platform-action-monitor-client"
    "github.com/eoscanada/eos-go"
    "github.com/stretchr/testify/assert"
    "net/http/httptest"
    "os"
    "testing"
    "net/http"
)

var a *App

const (
    depositPk     = "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAbuatmU"
    signiDicePk   = "5Jt3SSiedaGmkXGxxW1rc65vFjt2xm8RTEfWCnZoZGxanBg3SdY"
    chainID       = "cda75f235aef76ad91ef0503421514d80d8dbb584cd07178022f0bc7deb964ff"
    casinoAccName = "casino.one"
)

func MakeTestConfig() *AppConfig {
    keyBag := eos.KeyBag{}
    if err := keyBag.Add(depositPk); err != nil {
        panic(err)
    }
    if err := keyBag.Add(signiDicePk); err != nil {
        panic(err)
    }
    pubKeys, _ := keyBag.AvailableKeys()
    return &AppConfig{
        Broker {0, 0 },
        BlockChain{
            eos.Checksum256(chainID),
            casinoAccName,
            PubKeys{pubKeys[0], pubKeys[1]},
            nil,
        },
    }
}

func TestMain(m *testing.M) {
    InitLogger("debug")
    events := make(chan *broker.EventMessage)
    listener := broker.NewEventListener(":8888", events)
    f := &bytes.Buffer{}
    a = NewApp(eos.New("https://api.daobet.org"), listener, events, f, MakeTestConfig())
    code := m.Run()
    os.Exit(code)
}

func TestPingQuery(t *testing.T) {
    assert := assert.New(t)

    request, _ := http.NewRequest("GET", "/ping", nil)
    response := httptest.NewRecorder()

    a.PingQuery(response, request)

    assert.Equal(response.Body.String(), "{\"result\":\"pong\"}", "/ping failed")
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
    request, _ := http.NewRequest("POST", "/sign_transaction", bytes.NewBuffer(rawTransaction))
    response := httptest.NewRecorder()

    a.SignQuery(response, request)

    assert.Equal(response.Body.String(), `{"error":"failed to deserialize transaction"}`)
}