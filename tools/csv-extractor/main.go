package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	argDBAddress := flag.String("mongo", "", "mongodb address")
	argDBName := flag.String("dbname", "coin", "db name")
	flag.Parse()
	// mongodb://root:example@0.0.0.0:8041
	err := connectDB(*argDBName, *argDBAddress)
	if err != nil {
		panic(err)
	}
	// processTrade()
	// fmt.Println("done processTrade")
	processContribute()
	fmt.Println("done processContribute")
	// processWithdraw()
	// fmt.Println("done processWithdraw")
}

func processTrade() {
	csvTradeFile, err := os.Create("./trade.csv")

	if err != nil {
		fmt.Println(err)
	}
	defer csvTradeFile.Close()
	d := TradeCSV{}
	d.CSVheader(csvTradeFile)
	offset := int64(0)
	for {
		list, err := getTradeCSV(offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			return
		}
		for _, v := range list {
			v.CSVrow(csvTradeFile)
		}
		offset += int64(len(list))
	}
}

func processContribute() {
	csvTradeFile, err := os.Create("./contribute.csv")

	if err != nil {
		fmt.Println(err)
	}
	defer csvTradeFile.Close()
	d := ContributeCSV{}
	d.CSVheader(csvTradeFile)
	offset := int64(0)
	for {
		list, err := getTxContribute(offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			return
		}
		for _, v := range list {
			v.CSVrow(csvTradeFile)
		}
		offset += int64(len(list))
	}
}

func processWithdraw() {
	csvTradeFile, err := os.Create("./withdraw.csv")

	if err != nil {
		fmt.Println(err)
	}
	defer csvTradeFile.Close()
	d := WithdrawCSV{}
	d.CSVheader(csvTradeFile)
	offset := int64(0)
	for {
		list, err := getTxWithdraw(offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			return
		}
		for _, v := range list {
			v.CSVrow(csvTradeFile)
		}
		offset += int64(len(list))
	}
}

func connectDB(dbName string, mongoAddr string) error {
	err := mgm.SetDefaultConfig(nil, dbName, options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return err
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err = cd.Ping(context.Background(), nil)
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func getTradeCSV(offset int64) ([]TradeCSV, error) {
	limit := int64(10000)
	var result []TradeCSV
	var tradeSuccess []shared.TradeData
	filter := bson.M{"status": bson.M{operator.In: []string{"accepted", "xPoolTradeAccepted"}}}
	err := mgm.Coll(&shared.TradeData{}).SimpleFind(&tradeSuccess, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Limit: &limit,
		Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}
	if len(tradeSuccess) == 0 {
		return nil, nil
	}

	for _, v := range tradeSuccess {
		data := TradeCSV{
			TxRequest: v.RequestTx,
			TxRespond: v.RespondTx,
			Receive:   v.Amount,
			BuyToken:  v.TokenID,
		}
		txs, err := database.DBGetTxByHash([]string{v.RequestTx})
		if err != nil {
			panic(err)
		}

		if txs[0].Metatype == strconv.Itoa(metadata.PDETradeRequestMeta) {
			md := metadata.PDETradeRequest{}
			err := json.Unmarshal([]byte(txs[0].Metadata), &md)
			if err != nil {
				panic(err)
			}
			data.Amount = md.SellAmount
			data.TradingFee = md.TradingFee
			data.MinAcceptableReceiver = md.MinAcceptableAmount
			data.SellToken = md.TokenIDToSellStr
			data.User = md.TraderAddressStr
		}
		if txs[0].Metatype == strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta) {
			md := metadata.PDECrossPoolTradeRequest{}
			err := json.Unmarshal([]byte(txs[0].Metadata), &md)
			if err != nil {
				panic(err)
			}
			data.Amount = md.SellAmount
			data.TradingFee = md.TradingFee
			data.MinAcceptableReceiver = md.MinAcceptableAmount
			data.SellToken = md.TokenIDToSellStr
			data.User = md.TraderAddressStr
		}
		result = append(result, data)
	}

	return result, nil
}

func getTxContribute(offset int64) ([]ContributeCSV, error) {
	limit := int64(10000)
	var result []ContributeCSV
	var ctrbSuccess []shared.ContributionData
	filter := bson.M{"status": bson.M{operator.In: []string{"waiting", "matched", "matchedNReturned"}}}
	err := mgm.Coll(&shared.ContributionData{}).SimpleFind(&ctrbSuccess, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Limit: &limit,
		Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}
	resultMap := make(map[string]ContributeCSV)

	for _, v := range ctrbSuccess {
		data := ContributeCSV{
			TxRequests:         []string{v.RequestTx},
			PairID:             v.PairID,
			TokenID1:           v.TokenID,
			Token1Amount:       v.Amount,
			Token1AmountReturn: v.ReturnAmount,
			User:               v.ContributorAddressStr,
			status:             v.Status,
		}

		if d, ok := resultMap[data.PairID]; !ok {
			if v.RespondTx != "" {
				data.TxResponds = append(data.TxResponds, v.RespondTx)
			}
			resultMap[data.PairID] = data
		} else {
			willAddReq := true
			for _, tx := range d.TxRequests {
				if tx == v.RequestTx {
					willAddReq = false
					break
				}
			}
			if willAddReq {
				d.TxRequests = append(d.TxRequests, v.RequestTx)
			}
			if d.TokenID1 == data.TokenID1 {
				d.TokenID1 = data.TokenID1
				d.Token1Amount = data.Token1Amount
				d.Token1AmountReturn = data.Token1AmountReturn
			} else {
				d.TokenID2 = data.TokenID1
				d.Token2Amount = data.Token1Amount
				d.Token2AmountReturn = data.Token1AmountReturn
			}

			if v.RespondTx != "" {
				d.TxResponds = append(d.TxResponds, v.RespondTx)
			}
			if v.Status == "matched" || v.Status == "matchedNReturned" {
				d.status = v.Status
			}
			resultMap[data.PairID] = d
		}
	}

	for _, v := range resultMap {
		if v.status == "matched" || v.status == "matchedNReturned" {
			result = append(result, v)
		}
	}

	return result, nil
}

func getTxWithdraw(offset int64) ([]WithdrawCSV, error) {
	limit := int64(10000)
	var result []WithdrawCSV
	var wdSuccess []shared.WithdrawContributionData
	filter := bson.M{"status": bson.M{operator.Eq: "accepted"}}
	err := mgm.Coll(&shared.WithdrawContributionData{}).SimpleFind(&wdSuccess, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Limit: &limit,
		Skip:  &offset,
	})
	if err != nil {
		return nil, err
	}

	if len(wdSuccess) == 0 {
		return nil, nil
	}

	for _, v := range wdSuccess {
		data := WithdrawCSV{
			TxRequest:    v.RequestTx,
			TxResponds:   v.RespondTx,
			TokenID1:     v.TokenID1,
			TokenID2:     v.TokenID2,
			User:         v.ContributorAddressStr,
			Token1Amount: v.Amount1,
			Token2Amount: v.Amount2,
		}
		txs, err := database.DBGetTxByHash([]string{v.RequestTx})
		if err != nil {
			panic(err)
		}

		if txs[0].Metatype == strconv.Itoa(metadata.PDEWithdrawalRequestMeta) {
			md := metadata.PDEWithdrawalRequest{}
			err := json.Unmarshal([]byte(txs[0].Metadata), &md)
			if err != nil {
				panic(err)
			}
			data.Share = md.WithdrawalShareAmt
		}
		result = append(result, data)
	}

	return result, nil
}
