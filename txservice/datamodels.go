package main

import "github.com/kamva/mgm/v3"

type TxData struct {
	mgm.DefaultModel `bson:",inline"`
	Raw              string `json:"raw" bson:"raw"`
	TxHash           string `json:"txhash" bson:"txhash"`
	Status           string `json:"status" bson:"status"`
	Error            string `json:"error" bson:"error"`
}

func NewTxData(txhash, raw, status, err string) *TxData {
	return &TxData{TxHash: txhash, Raw: raw, Status: status, Error: err}
}

func (model *TxData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TxData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}
