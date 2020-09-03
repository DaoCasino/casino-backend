// main_test.go

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/eoscanada/eos-go/ecc"

	"github.com/DaoCasino/casino-backend/mocks"
	broker "github.com/DaoCasino/platform-action-monitor-client"
	"github.com/eoscanada/eos-go"
	"github.com/stretchr/testify/assert"
)

var a *App

const (
	bcURL           = "localhost:8888"
	depositPk       = "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAbuatmU"
	signiDicePk     = "5KXQYCyytPBsKoymLuDjmg1MdqeSUmFRiczGe67HdWdvuBggKyS"
	chainID         = "cda75f235aef76ad91ef0503421514d80d8dbb584cd07178022f0bc7deb964ff"
	casinoAccName   = "daocasinoxxx"
	platformAccName = "platform"
	platformPk      = "5KUc6M7hzDr63kDsn2iLn54X7JpzYyXtUEc5iuqieRkQp4iYYkv"
)

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
	platformKey, _ := ecc.NewPrivateKey(platformPk)
	return &AppConfig{
		BrokerConfig{0, 0},
		BlockChainConfig{
			eos.Checksum256(chainID),
			casinoAccName,
			PubKeys{pubKeys[0], pubKeys[1]},
			rsaKey,
			platformAccName,
			platformKey.PublicKey(),
		},
		HTTPConfig{3, 3 * time.Second, 3 * time.Second},
	}, &keyBag
}

func TestMain(m *testing.M) {
	InitLogger("debug")
	events := make(chan *broker.EventMessage)
	listener := new(mocks.EventListenerMock)
	f := &mocks.SafeBuffer{}
	appCfg, keyBag := MakeTestConfig()
	bc := eos.New(bcURL)
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

func TestSignidiceAction(t *testing.T) {
	assert := assert.New(t)
	action := NewSigndice("gamesc", "onecasino", 42, "casinosig")
	assert.Equal(eos.AN("gamesc"), action.Account)
	assert.Equal(eos.ActionName("sgdicesecond"), action.Name)
	assert.Equal([]eos.PermissionLevel{
		{Actor: eos.AN("onecasino"), Permission: eos.PN("active")},
	},
		action.Authorization)
	assert.Equal(eos.NewActionData(Signidice{RequestID: 42, Signature: "casinosig"}), action.ActionData)
}

func TestSignidiceTransaction(t *testing.T) {
	assert := assert.New(t)
	dicePubKey := a.BlockChain.EosPubKeys.SigniDice
	txOpts := &eos.TxOptions{ChainID: eos.Checksum256(chainID)}
	packedTx, err := GetSigndiceTransaction(a.bcAPI, "gamesc", "onecasino",
		42, "casinosig", dicePubKey, txOpts)
	assert.Nil(err)
	signedTx, err := packedTx.Unpack()
	assert.Nil(err)

	pubKeys, err := signedTx.SignedByKeys(eos.Checksum256(chainID))
	assert.Nil(err)
	assert.Equal(1, len(pubKeys))
	assert.Equal(dicePubKey, pubKeys[0])
}

func TestValidateTransaction(t *testing.T) {
	assert := assert.New(t)
	sponsorPk := "5J6wt29qMkX2d22x2dw7QQb2S7A9c9xjrSiA16t6TAwTNqntpi1"
	keyBag := eos.KeyBag{}
	err := keyBag.Add(sponsorPk)
	assert.Nil(err)
	err = keyBag.Add(platformPk)
	assert.Nil(err)
	err = keyBag.Add(signiDicePk)
	assert.Nil(err)
	pubKeys, _ := keyBag.AvailableKeys()
	transferAction := &eos.Action{
		Account: eos.AN("eosio.token"),
		Name:    eos.ActN("transfer"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN("player"), Permission: eos.PN(casinoAccName)},
		},
	}
	newGameAction := &eos.Action{
		Account: eos.AN("dice"),
		Name:    eos.ActN("newgame"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(platformAccName), Permission: eos.PN("gameaction")},
		},
	}
	gameActionAction := &eos.Action{
		Account: eos.AN("dice"),
		Name:    eos.ActN("gameaction"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(platformAccName), Permission: eos.PN("gameaction")},
		},
	}
	assert.Nil(ValidateTransferAction(transferAction, eos.AN(casinoAccName)))
	assert.Equal(ValidateTransferAction(transferAction, eos.AN("onebet")),
		fmt.Errorf("invalid permission in transfer action"))
	assert.Nil(ValidateGameActionAuth(newGameAction, eos.AN(platformAccName)))
	assert.Equal(ValidateGameActionAuth(newGameAction, eos.AN("buggyplatform")),
		fmt.Errorf("invalid actor in game action"))
	assert.Nil(ValidateGameActionAuth(gameActionAction, eos.AN(platformAccName)))
	assert.Equal(ValidateGameActionAuth(gameActionAction, eos.AN("buggyplatform")),
		fmt.Errorf("invalid actor in game action"))

	// {transfer, newgame} ok
	txn := *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, newGameAction}, nil))
	origTxn := txn
	signedTxn, err := keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Nil(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)))

	// {transfer, newgame} invalid keys
	nonPlatformTxn, err := keyBag.Sign(&origTxn, eos.Checksum256(chainID), pubKeys[0], pubKeys[2])
	assert.Nil(err)
	assert.Equal(ValidateDepositTransaction(nonPlatformTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)),
		fmt.Errorf("platform pub key not found in deposit txn"))

	// {transfer, gameaction} ok
	txn = *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, gameActionAction}, nil))
	signedTxn, err = keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Nil(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)))

	// {transfer, newgame, gameaction} ok
	txn = *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, newGameAction, gameActionAction}, nil))
	signedTxn, err = keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Nil(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)))

	// {transfer, newgame, newgame} invalid
	txn = *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, newGameAction, newGameAction}, nil))
	signedTxn, err = keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Equal(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)),
		fmt.Errorf("first action should be newgame, second gameaction"))

	// {transfer, gameaction, newgame} invalid
	txn = *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, newGameAction, newGameAction}, nil))
	signedTxn, err = keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Equal(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)),
		fmt.Errorf("first action should be newgame, second gameaction"))

	// {transfer, gameaction, gameaction} invalid
	txn = *eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{transferAction, newGameAction, newGameAction}, nil))
	signedTxn, err = keyBag.Sign(&txn, eos.Checksum256(chainID), pubKeys[0], pubKeys[1])
	assert.Nil(err)
	assert.Equal(ValidateDepositTransaction(signedTxn,
		eos.AN(casinoAccName), eos.AN(platformAccName),
		a.BlockChain.PlatformPubKey,
		eos.Checksum256(chainID)),
		fmt.Errorf("first action should be newgame, second gameaction"))
}
