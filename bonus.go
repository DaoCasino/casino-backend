package main

import (
	"github.com/eoscanada/eos-go"
)

type PlayerStats struct {
	Player          string    `json:"player"`
	SessionsCreated uint64    `json:"sessions_created"`
	VolumeReal      eos.Asset `json:"volume_real"`
	VolumeBonus     eos.Asset `json:"volume_bonus"`
	ProfitReal      eos.Asset `json:"profit_real"`
	ProfitBonus     eos.Asset `json:"profit_bonus"`
}

type PlayerBalance struct {
	Player  string    `json:"player"`
	Balance eos.Asset `json:"balance"`
}

func (app *App) getBonusPlayersStats() ([]PlayerStats, error) {
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

func (app *App) getBonusPlayersBalance() ([]PlayerBalance, error) {
	resp, err := app.bcAPI.GetTableRows(eos.GetTableRowsRequest{
		Code:  string(app.BlockChain.CasinoAccountName),
		Scope: string(app.BlockChain.CasinoAccountName),
		Table: "bonusbalance",
		Limit: 0,
		JSON:  true,
	})
	if err != nil {
		return nil, err
	}

	var playersBalance []PlayerBalance

	err = resp.JSONToStructs(&playersBalance)
	if err != nil {
		return nil, err
	}

	return playersBalance, nil
}
