// main_test.go

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/eoscanada/eos-go"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var a *App

const (
	depositPk     = "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAbuatmU"
	signiDicePk   = "5KXQYCyytPBsKoymLuDjmg1MdqeSUmFRiczGe67HdWdvuBggKyS"
	chainID       = "cda75f235aef76ad91ef0503421514d80d8dbb584cd07178022f0bc7deb964ff"
	casinoAccName = "daocasinoxxx"
)

type EventListenerMock struct{}

func (e *EventListenerMock) ListenAndServe(ctx context.Context) error {
	return nil
}

func (e *EventListenerMock) Subscribe(eventType broker.EventType, offset uint64) (bool, error) {
	return true, nil
}

func (e *EventListenerMock) Unsubscribe(eventType broker.EventType) (bool, error) {
	return true, nil
}

func MakeTestConfig() (*AppConfig, *eos.KeyBag) {
	keyBag := eos.KeyBag{}
	if err := keyBag.Add(depositPk); err != nil {
		panic(err)
	}
	if err := keyBag.Add(signiDicePk); err != nil {
		panic(err)
	}
	pubKeys, _ := keyBag.AvailableKeys()
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	return &AppConfig{
		Broker{0, 0},
		BlockChain{
			eos.Checksum256(chainID),
			casinoAccName,
			PubKeys{pubKeys[0], pubKeys[1]},
			rsaKey,
		},
	}, &keyBag
}

func TestMain(m *testing.M) {
	InitLogger("debug")
	events := make(chan *broker.EventMessage)
	listener := new(EventListenerMock)
	f := &bytes.Buffer{}
	appCfg, keyBag := MakeTestConfig()
	bc := eos.New("https://api.daobet.org")
	bc.SetSigner(keyBag)
	a = NewApp(bc, listener, events, f, appCfg)
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

func TestSignidice(t *testing.T) {
	assert := assert.New(t)
	sha256String := sha256.Sum256([]byte("A"))
	strToSign := hex.EncodeToString(sha256String[:])
	data := json.RawMessage(fmt.Sprintf(`{"digest": "%v"}`, strToSign))
	events := []*broker.Event{
		{
			Offset: 0,
			Sender: "daoplayeryyy",
			CasinoID: 42,
			GameID: 42,
			RequestID: 42,
			EventType: 42,
			Data: data,
		},
	}

	txID := a.processEvent(events[0])
	assert.NotNil(txID)
	tx, err := a.bcAPI.GetTransaction(*txID)
	assert.Nil(err)
	actions := tx.Transaction.Transaction.Actions
	assert.Equal(1, len(actions))
	assert.Equal("sgdicesecond", string(actions[0].Name))
	assert.Equal("daoplayeryyy", string(actions[0].Account))
	assert.Equal([]eos.PermissionLevel{
		{
			Actor:      eos.AN(casinoAccName),
			Permission: "signidice",
		},
	}, actions[0].Authorization)
}

func TestSignidiceMultipleGoroutines(t *testing.T) {
	assert := assert.New(t)
	sha256String := sha256.Sum256([]byte("A"))
	strToSign := hex.EncodeToString(sha256String[:])
	data := json.RawMessage(fmt.Sprintf(`{"digest": "%v"}`, strToSign))
	events := []*broker.Event{
		{
			Offset: 0,
			Sender: "daoplayeryyy",
			CasinoID: 1,
			GameID: 1,
			RequestID: 1,
			EventType: 1,
			Data: data,
		},
		{
			Offset: 1,
			Sender: "daoplayeryyy",
			CasinoID: 2,
			GameID: 2,
			RequestID: 2,
			EventType: 2,
			Data: data,
		},
	}

	logBuffer := &bytes.Buffer{}
	oldLogger := log.Logger
	log.Logger = log.Logger.Output(logBuffer)
	defer func() {
		log.Logger = oldLogger
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.RunEventProcessor(ctx)

	a.EventMessages <- &broker.EventMessage{Offset: 1, Events: events}
	time.Sleep(2 * time.Second)

	offsetHandler, _ := a.OffsetHandler.(*bytes.Buffer)
	assert.Equal(offsetHandler.String(), "2")

	txnsSent := 0

	for _, logLine := range strings.Split(logBuffer.String(), "\n") {
		txOkMessage := "Successfully signed and sent txn, id: "
		if idx := strings.LastIndex(logLine, txOkMessage); idx != -1 {
			txnsSent++
			// get tx id from log message
			txID := logLine[idx+len(txOkMessage) : idx+len(txOkMessage)+64]
			fmt.Println(txID)
			_, err := a.bcAPI.GetTransaction(txID)
			assert.Nil(err)
		}
	}
	assert.Equal(2, txnsSent)
}
