package main

import (
    "github.com/eoscanada/eos-go"
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
) (*eos.SignedTransaction, *eos.PackedTransaction, error) {
    action := NewSigndice(contract, casinoAccount, requestID, signature)
    txOpts := &eos.TxOptions{}

    if err := txOpts.FillFromChain(api); err != nil {
        return nil, nil, err
    }
    tx := eos.NewTransaction([]*eos.Action{action}, txOpts)
    // eos-go will automatically sign with required key
    signedTx, packedTx, err := api.SignTransaction(tx, txOpts.ChainID, eos.CompressionNone)
    if err != nil {
        return nil, nil, err
    }
    return signedTx, packedTx, nil
}
