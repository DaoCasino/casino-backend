package main

import (
	"fmt"
	"github.com/DaoCasino/casino-backend/utils"
	"github.com/eoscanada/eos-go"
	"github.com/rs/zerolog/log"
	"strconv"
)

type PlayerStats struct {
	Player          string    `json:"player"`
	SessionsCreated uint64    `json:"sessions_created"`
	VolumeReal      eos.Asset `json:"volume_real"`
	VolumeBonus     eos.Asset `json:"volume_bonus"`
	ProfitReal      eos.Asset `json:"profit_real"`
	ProfitBonus     eos.Asset `json:"profit_bonus"`
}

type ConvertBonusData struct {
	Name eos.AccountName `json:"name"`
	Memo string          `json:"memo"`
}

func (app *App) getBonusPlayers() ([]PlayerStats, error) {
	resp, err := app.bcAPI.GetTableRows(eos.GetTableRowsRequest{
		Code:  string(app.BlockChain.CasinoAccountName),
		Scope: string(app.BlockChain.CasinoAccountName),
		Table: "playerstats",
		Limit: 0,
		JSON:  true,
	})
	if err != nil {
		return nil, err
	}

	var playerStats []PlayerStats

	err = resp.JSONToStructs(&playerStats)
	if err != nil {
		return nil, err
	}

	return playerStats, nil
}

func (app *App) convertBonus(player string, force bool) error {
	if !force {
		meetRequirements, err := app.meetRequirements(player)
		if err != nil {
			return fmt.Errorf("failed to check player meet requirements: %w", err)
		}

		if !meetRequirements {
			return fmt.Errorf("player doesn't meet requirements")
		}
	}

	var txOpts *eos.TxOptions
	if err := utils.RetryWithTimeout(func() error {
		var e error
		txOpts, e = app.getTxOpts()
		return e
	}, app.HTTP.RetryAmount, app.HTTP.Timeout, app.HTTP.RetryDelay); err != nil {
		return fmt.Errorf("failed to get blockchain state: %w", err)
	}

	action := &eos.Action{
		Account: app.BlockChain.CasinoAccountName,
		Name:    eos.ActN("convertbon"),
		Authorization: []eos.PermissionLevel{
			{Actor: app.BlockChain.BonusAdminAccountName, Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(ConvertBonusData{
			Name: eos.AN(player),
			Memo: "",
		}),
	}

	tx := eos.NewSignedTransaction(eos.NewTransaction([]*eos.Action{action}, txOpts))
	signedTx, err := app.bcAPI.Signer.Sign(tx, txOpts.ChainID, app.BlockChain.EosPubKeys.SigniDice)
	if err != nil {
		return fmt.Errorf("failed to sign trx: %w", err)
	}
	log.Debug().Msg(signedTx.String())

	packedTrx, err := tx.Pack(eos.CompressionNone)
	if err != nil {
		return fmt.Errorf("failed to pack trx: %w", err)
	}

	trxID, err := packedTrx.ID()
	if err != nil {
		return fmt.Errorf("failed to calc trx ID: %w", err)
	}
	trxHexEncoded := trxID.String()
	if err := SendPackedTrxWithRetries(app.bcAPI, packedTrx, trxHexEncoded,
		app.HTTP.RetryAmount, app.HTTP.Timeout, app.HTTP.RetryDelay); err != nil {
		return fmt.Errorf("failed to send convert bonus trx: %w", err)
	}

	return nil
}

func (app *App) meetRequirements(player string) (bool, error) {
	resp, err := app.bcAPI.GetTableRows(eos.GetTableRowsRequest{
		Code:       string(app.BlockChain.CasinoAccountName),
		Scope:      string(app.BlockChain.CasinoAccountName),
		Table:      "playerstats",
		LowerBound: strconv.FormatUint(eos.MustStringToName(player), 10),
		Limit:      1,
		JSON:       true,
	})
	if err != nil {
		return false, err
	}

	playerStats := make([]PlayerStats, 1)

	err = resp.JSONToStructs(&playerStats)
	if err != nil {
		return false, err
	}

	// TODO check requirements

	return true, nil
}
