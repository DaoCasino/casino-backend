package main

import (
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
)

func NewSigndice(contract, casinoAccount string, requestID uint64, signature string) *eos.Action {
	return &eos.Action{
		Account: eos.AN(contract),
		Name:    eos.ActN("sgdicesecond"),
		Authorization: []eos.PermissionLevel{
			{Actor: eos.AN(casinoAccount), Permission: eos.PN("signidice")},
		},
		ActionData: eos.NewActionData(Signidice{
			requestID,
			signature,
		}),
	}
}

// Game contract's sgdicesecond action parameters
type Signidice struct {
	RequestID uint64 `json:"req_id"`
	Signature string `json:"sign"`
}


func GetSigndiceTransaction(api *eos.API, contract, casinoAccount string, requestID uint64, signature ecc.Signature) (*eos.SignedTransaction, *eos.PackedTransaction) {
	action := NewSigndice(contract, casinoAccount, requestID, string(signature.Content))
	txOpts := &eos.TxOptions{}

	if err := txOpts.FillFromChain(api); err != nil {
		panic(fmt.Errorf("filling tx opts: %s", err))
	}
	tx := eos.NewTransaction([]*eos.Action{action}, txOpts)
	signedTx, packedTx, err := api.SignTransaction(tx, txOpts.ChainID, eos.CompressionNone)
	if err != nil {
		panic(fmt.Errorf("sign transaction: %s", err))
	}
	return signedTx, packedTx
}
