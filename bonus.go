package main

import (
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
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
		meetRequirements, err := PlayerMeetRequirements(player, app.BlockChain.CasinoAccountName, app.bcAPI)
		if err != nil {
			return fmt.Errorf("failed to check player meet requirements: %w", err)
		}

		if !meetRequirements {
			return fmt.Errorf("player doesn't meet requirements")
		}
	}

	action := &eos.Action{
		Account: app.BlockChain.CasinoAccountName,
		Name:    eos.ActN("convertbon"),
		Authorization: []eos.PermissionLevel{
			{Actor: app.Bonus.AdminAccountName, Permission: eos.PN("active")},
		},
		ActionData: eos.NewActionData(ConvertBonusData{
			Name: eos.AN(player),
			Memo: "",
		}),
	}

	if err := app.PushTransaction([]*eos.Action{action}, []ecc.PublicKey{app.BlockChain.EosPubKeys.BonusAdmin}); err != nil {
		return fmt.Errorf("failed to push transaction: %w", err)
	}

	return nil
}

func PlayerMeetRequirements(player string, casino eos.AccountName, bcAPI *eos.API) (bool, error) {
	resp, err := bcAPI.GetTableRows(eos.GetTableRowsRequest{
		Code:       string(casino),
		Scope:      string(casino),
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
