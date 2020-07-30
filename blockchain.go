package main

import (
	"fmt"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/rs/zerolog/log"
)

func NewSigndice(contract, casinoAccount eos.AccountName, requestID uint64, signature string) *eos.Action {
	return &eos.Action{
		Account: contract,
		Name:    eos.ActN("sgdicesecond"),
		Authorization: []eos.PermissionLevel{
			{Actor: casinoAccount, Permission: eos.PN("signidice")},
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
	contract, casinoAccount eos.AccountName,
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

func ValidateDepositTransaction(
	tx *eos.SignedTransaction,
	casinoName, platformName eos.AccountName,
	platformPubKey ecc.PublicKey,
	chainID eos.Checksum256) error {
	if len(tx.Actions) != 2 {
		return fmt.Errorf("invalid actions size")
	}
	transferAction := tx.Actions[0]
	if err := ValidateTransferAction(transferAction, casinoName); err != nil {
		return err
	}
	if err := ValidateGameAction(tx.Actions[1], platformName); err != nil {
		return err
	}

	pubKeys, err := tx.SignedByKeys(chainID)
	log.Debug().Msgf("Deposit txn pubkeys: %v", pubKeys)
	if err != nil {
		return fmt.Errorf("failed to retrieve public keys from deposit transaction")
	}
	if err := ValidateSignatures(pubKeys, platformPubKey); err != nil {
		return err
	}
	return nil
}

func ValidateTransferAction(action *eos.Action, casinoName eos.AccountName) error {
	if action.Account != eos.AN("eosio.token") {
		return fmt.Errorf("invalid contract name in transfer action")
	}
	if action.Name != eos.ActN("transfer") {
		return fmt.Errorf("invalid action name in transfer action")
	}
	if len(action.Authorization) != 1 {
		return fmt.Errorf("invalid authorization size in transfer action")
	}
	if string(action.Authorization[0].Permission) != string(casinoName) {
		return fmt.Errorf("invalid permission in transfer action")
	}
	return nil
}

func ValidateGameAction(action *eos.Action, platformName eos.AccountName) error {
	if action.Name != eos.ActN("newgame") && action.Name != eos.ActN("gameaction") {
		return fmt.Errorf("invalid action name in game action")
	}
	if len(action.Authorization) != 1 {
		return fmt.Errorf("invalid authorization size in game action")
	}
	if action.Authorization[0].Actor != platformName {
		return fmt.Errorf("invalid actor in game action")
	}
	if action.Authorization[0].Permission != eos.PN("gameaction") {
		return fmt.Errorf("invalid permission name in game action")
	}
	return nil
}

func ValidateSignatures(pubKeys []ecc.PublicKey, platformPubKey ecc.PublicKey) error {
	// there are can be up to 3 signatures (platform deposit, platform gameaction, sponsor[optionally])
	if len(pubKeys) != 2 && len(pubKeys) != 3 {
		return fmt.Errorf("invalid signatures size in deposit txn")
	}
	for i := range pubKeys {
		if pubKeys[i].String() == platformPubKey.String() {
			return nil
		}
	}
	return fmt.Errorf("platform pub key not found in deposit txn")
}
