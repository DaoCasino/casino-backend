package main

import (
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/rs/zerolog/log"
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

func GetSigndiceTransaction(
	api *eos.API,
	contract, casinoAccount string,
	requestID uint64, signature string,
	signidiceKey ecc.PublicKey,
	txOpts *eos.TxOptions,
) (*eos.PackedTransaction, error) {
	action := NewSigndice(contract, casinoAccount, requestID, signature)
	tx := eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{action}, txOpts))
	signedTx, err := api.Signer.Sign(tx, txOpts.ChainID, signidiceKey)
	if err != nil {
		return nil, err
	}
	log.Debug().Msg(signedTx.String())
	return tx.Pack(eos.CompressionNone)
}
