package main

import (
	"fmt"
	"time"

	"github.com/DaoCasino/casino-backend/utils"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/rs/zerolog/log"
)

func NewSigndice(contract, signerAccount eos.AccountName, requestID uint64, signature string) *eos.Action {
	return &eos.Action{
		Account: contract,
		Name:    eos.ActN("sgdicesecond"),
		Authorization: []eos.PermissionLevel{
			{Actor: signerAccount, Permission: eos.PN("active")},
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
	contract, signerAccount eos.AccountName,
	requestID uint64, signature string,
	signidiceKey ecc.PublicKey,
	txOpts *eos.TxOptions,
) (*eos.PackedTransaction, error) {
	action := NewSigndice(contract, signerAccount, requestID, signature)
	tx := eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{action}, txOpts))
	signedTx, err := api.Signer.Sign(tx, txOpts.ChainID, signidiceKey)
	if err != nil {
		return nil, err
	}
	log.Debug().Msg(signedTx.String())
	return tx.Pack(eos.CompressionNone)
}

// allowed only 3 invariants: {transfer, newgame}, {transfer, gameaction}, {transfer, newgame, gameaction}
func ValidateDepositTransaction(
	tx *eos.SignedTransaction,
	casinoName, platformName eos.AccountName,
	platformPubKey ecc.PublicKey,
	chainID eos.Checksum256) error {
	if len(tx.Actions) != 2 && len(tx.Actions) != 3 {
		return fmt.Errorf("invalid actions size")
	}

	transferAction := tx.Actions[0] // first action always is transfer
	if err := ValidateTransferAction(transferAction, casinoName); err != nil {
		return err
	}

	// just validate second action authority
	if err := ValidateGameActionAuth(tx.Actions[1], platformName); err != nil {
		return err
	}

	if len(tx.Actions) == 2 { // if newgame or gameaction (1 and 2 invariants)
		if !isNewGame(tx.Actions[1]) && !isGameAction(tx.Actions[1]) {
			return fmt.Errorf("allowed only gameaction or newgame")
		}
	} else { // if gameaction and newgame at same time (3 invariant)
		// just validate additional action authority
		if err := ValidateGameActionAuth(tx.Actions[2], platformName); err != nil {
			return err
		}

		// first action should be newgame, second gameaction
		if !isNewGame(tx.Actions[1]) || !isGameAction(tx.Actions[2]) {
			return fmt.Errorf("first action should be newgame, second gameaction")
		}
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

func ValidateGameActionAuth(action *eos.Action, platformName eos.AccountName) error {
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

func isNewGame(action *eos.Action) bool {
	return action.Name == eos.ActN("newgame") || action.Name == eos.ActN("newgameaffl")
}

func isGameAction(action *eos.Action) bool {
	return action.Name == eos.ActN("gameaction")
}

func SendPackedTrxWithRetries(bcAPI *eos.API, packedTrx *eos.PackedTransaction, trxID string,
	retries int, timeout, retryDelay time.Duration) error {
	return utils.RetryWithTimeout(func() error {
		var e error
		_, e = bcAPI.PushTransaction(packedTrx)
		if e != nil {
			if apiErr, ok := e.(eos.APIError); ok {
				// if error is duplicate trx assume as OK
				if apiErr.Code == EosInternalErrorCode && apiErr.ErrorStruct.Code == EosInternalDuplicateErrorCode {
					log.Debug().Msgf("Got duplicate trx error, assuming as OK, trx_id: %s", trxID)
					return nil
				}
			}
		}
		return e
	}, retries, timeout, retryDelay)
}
