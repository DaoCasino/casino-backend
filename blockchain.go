package main

import (
	"fmt"
	"time"

	"github.com/DaoCasino/casino-backend/utils"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/rs/zerolog/log"
)

var (
	allowedInvariants = [][]string{
		{"transfer", "newgame"},
		{"transfer", "newgame", "gameaction"},
		{"transfer", "gameaction"},
		{"transfer", "newgamebon", "gameaction"},
		{"newgamebon", "gameaction"},
		{"transfer", "depositbon", "gameaction"},
		{"depositbon", "gameaction"},
	}
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

// allowed only 7 invariants: {transfer, newgame}, {transfer, newgame, gameaction}, {transfer, gameaction},
// {transfer, newgamebon, gameaction}, {newgamebon, gameaction}, {transfer, depositbon, gameaction},
// {depositbon, gameaction}

func ValidateDepositTransaction(
	tx *eos.SignedTransaction,
	casinoName, platformName eos.AccountName,
	platformPubKey ecc.PublicKey,
	chainID eos.Checksum256) error {
	if len(tx.Actions) != 2 && len(tx.Actions) != 3 {
		return fmt.Errorf("invalid actions size")
	}

	invariant, err := extractInvariant(tx.Actions)
	if err != nil {
		return err
	}

	log.Debug().Msgf("%+v", invariant)

	if !isInvariantAllowed(invariant) {
		return fmt.Errorf("incorrect tx actions")
	}

	for i, name := range invariant {
		if name == "transfer" {
			if err := ValidateTransferAction(tx.Actions[i], casinoName); err != nil {
				return err
			}
		} else {
			if err := ValidateGameActionAuth(tx.Actions[i], platformName); err != nil {
				return err
			}
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

func isTransfer(action *eos.Action) bool {
	return action.Name == eos.ActN("transfer")
}

func isDepositBon(action *eos.Action) bool {
	return action.Name == eos.ActN("depositbon")
}

func isNewGame(action *eos.Action) bool {
	return action.Name == eos.ActN("newgame") || action.Name == eos.ActN("newgameaffl")
}

func isNewGameBon(action *eos.Action) bool {
	return action.Name == eos.ActN("newgamebon")
}

func isGameAction(action *eos.Action) bool {
	return action.Name == eos.ActN("gameaction")
}

func getInvariantName(action *eos.Action) (string, error) {
	switch {
	case isTransfer(action):
		return "transfer", nil
	case isDepositBon(action):
		return "depositbon", nil
	case isNewGame(action):
		return "newgame", nil
	case isNewGameBon(action):
		return "newgamebon", nil
	case isGameAction(action):
		return "gameaction", nil
	}
	return "", fmt.Errorf("action is not allowed")
}

func extractInvariant(actions []*eos.Action) ([]string, error) {
	var invariant []string
	for _, act := range actions {
		name, err := getInvariantName(act)
		if err != nil {
			return nil, err
		}
		invariant = append(invariant, name)
	}
	return invariant, nil
}

func isInvariantAllowed(invariant []string) bool {
	for _, inv := range allowedInvariants {
		if len(inv) != len(invariant) {
			continue
		}

		matches := true

		for i := range inv {
			if inv[i] != invariant[i] {
				matches = false
				break
			}
		}

		if matches {
			return true
		}
	}

	return false
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
