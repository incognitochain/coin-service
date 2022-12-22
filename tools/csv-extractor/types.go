package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type TradeCSV struct {
	TxRequest             string
	TxRespond             string
	SellToken             string
	BuyToken              string
	Amount                uint64
	MinAcceptableReceiver uint64
	Receive               uint64
	TradingFee            uint64
	FeeToken              string
	User                  string
	Unix                  string
	FormatedDate          string
	IsSwap                bool
	TradePath             []string
	unixint               int64
}

func (*TradeCSV) CSVheader(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{"TxRequest", "TxRespond", "SellToken", "BuyToken", "Amount", "MinAcceptableReceiver", "Receive", "TradingFee", "FeeToken", "User", "Unix", "FormatedDate", "TradePath"})
	cw.Flush()
}

func (rm *TradeCSV) CSVrow(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{rm.TxRequest, rm.TxRespond, rm.SellToken, rm.BuyToken, fmt.Sprintf("%v", rm.Amount), fmt.Sprintf("%v", rm.MinAcceptableReceiver), fmt.Sprintf("%v", rm.Receive), fmt.Sprintf("%v", rm.TradingFee), rm.FeeToken, rm.User, rm.Unix, rm.FormatedDate, strings.Join(rm.TradePath, ",")})
	cw.Flush()
}

type ContributeCSV struct {
	TxRequests         []string
	TxResponds         []string
	PairID             string
	TokenID1           string
	TokenID2           string
	Token1Amount       uint64
	Token2Amount       uint64
	Token1AmountReturn uint64
	Token2AmountReturn uint64
	User               string
	status             string
	Unix               string
	FormatedDate       string
}

func (*ContributeCSV) CSVheader(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{"TxRequests", "TxResponds", "PairID", "TokenID1", "TokenID2", "Token1Amount", "Token2Amount", "Token1AmountReturn", "Token2AmountReturn", "User", "Unix", "FormatedDate"})
	cw.Flush()
}

func (rm *ContributeCSV) CSVrow(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{strings.Join(rm.TxRequests, ";"), strings.Join(rm.TxResponds, ";"), rm.PairID, rm.TokenID1, rm.TokenID2, fmt.Sprintf("%v", rm.Token1Amount), fmt.Sprintf("%v", rm.Token2Amount), fmt.Sprintf("%v", rm.Token1AmountReturn), fmt.Sprintf("%v", rm.Token2AmountReturn), rm.User, rm.Unix, rm.FormatedDate})
	cw.Flush()
}

type WithdrawCSV struct {
	TxRequest    string
	TxResponds   []string
	TokenID1     string
	TokenID2     string
	Token1Amount uint64
	Token2Amount uint64
	Share        uint64
	User         string
	Unix         string
	FormatedDate string
}

func (*WithdrawCSV) CSVheader(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{"TxRequest", "TxResponds", "TokenID1", "TokenID2", "Token1Amount", "Token2Amount", "Share", "User", "Unix", "FormatedDate"})
	cw.Flush()
}

func (rm *WithdrawCSV) CSVrow(w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{rm.TxRequest, strings.Join(rm.TxResponds, ";"), rm.TokenID1, rm.TokenID2, fmt.Sprintf("%v", rm.Token1Amount), fmt.Sprintf("%v", rm.Token2Amount), fmt.Sprintf("%v", rm.Share), rm.User, rm.Unix, rm.FormatedDate})
	cw.Flush()
}
