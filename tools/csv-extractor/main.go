package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

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
	argFromTime := flag.Int64("from", 0, "from time")
	argToTime := flag.Int64("to", 0, "to time")
	flag.Parse()
	// mongodb://root:example@0.0.0.0:8041
	err := connectDB(*argDBName, *argDBAddress)
	if err != nil {
		panic(err)
	}
	processTrade(*argFromTime, *argToTime)
	fmt.Println("done processTrade")
}

func processTrade(fromTime, toTime int64) {
	csvTradeFile, err := os.Create("./trade.csv")

	if err != nil {
		fmt.Println(err)
	}
	defer csvTradeFile.Close()
	d := TradeCSV{}
	d.CSVheader(csvTradeFile)
	offset := int64(0)
	for {
		list, err := getTradeCSV(fromTime, toTime, offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			return
		}
		for _, v := range list {
			v.CSVrow(csvTradeFile)
		}
		fmt.Println("processed", len(list))
		offset += 40000
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

func getTradeCSV(fromTime, toTime, offset int64) ([]TradeCSV, error) {
	limit := int64(40000)
	var result []TradeCSV
	var tradeSuccess []shared.TradeData
	var txList []shared.TxData
	var txHashs []string

	AllowDiskUse := true
	filter := bson.M{"locktime": bson.M{operator.Gte: fromTime, operator.Lt: toTime}, "metatype": bson.M{operator.In: []string{strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta), strconv.Itoa(metadata.PDETradeRequestMeta)}}}
	err := mgm.Coll(&shared.TxData{}).SimpleFind(&txList, filter, &options.FindOptions{
		Sort:         bson.D{{"locktime", 1}},
		AllowDiskUse: &AllowDiskUse,
		Limit:        &limit,
		Skip:         &offset,
	})
	if err != nil {
		return nil, err
	}
	if len(txList) == 0 {
		return nil, nil
	}
	for _, v := range txList {
		txHashs = append(txHashs, v.TxHash)
	}

	filter = bson.M{"status": bson.M{operator.In: []string{"accepted", "xPoolTradeAccepted"}}, "requesttx": bson.M{operator.In: txHashs}}
	err = mgm.Coll(&shared.TradeData{}).SimpleFind(&tradeSuccess, filter, &options.FindOptions{AllowDiskUse: &AllowDiskUse})
	if err != nil {
		return nil, err
	}
	if len(tradeSuccess) == 0 {
		return nil, nil
	}

	for _, v := range tradeSuccess {
		tx := shared.TxData{}
		for _, h := range txList {
			if v.RequestTx == h.TxHash {
				tx = h
				break
			}
		}
		timeText := time.Unix(tx.Locktime, 0).String()
		data := TradeCSV{
			TxRequest:    v.RequestTx,
			TxRespond:    v.RespondTx,
			Receive:      v.Amount,
			BuyToken:     v.TokenID,
			Unix:         fmt.Sprintf("%v", tx.Locktime),
			FormatedDate: timeText,
		}

		if tx.Metatype == strconv.Itoa(metadata.PDETradeRequestMeta) {
			md := metadata.PDETradeRequest{}
			err := json.Unmarshal([]byte(tx.Metadata), &md)
			if err != nil {
				panic(err)
			}
			data.Amount = md.SellAmount
			data.TradingFee = md.TradingFee
			data.MinAcceptableReceiver = md.MinAcceptableAmount
			data.SellToken = md.TokenIDToSellStr
			data.User = md.TraderAddressStr
		}
		if tx.Metatype == strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta) {
			md := metadata.PDECrossPoolTradeRequest{}
			err := json.Unmarshal([]byte(tx.Metadata), &md)
			if err != nil {
				panic(err)
			}
			data.Amount = md.SellAmount
			data.TradingFee = md.TradingFee
			data.MinAcceptableReceiver = md.MinAcceptableAmount
			data.SellToken = md.TokenIDToSellStr
			data.User = md.TraderAddressStr
		}
		data.unixint = tx.Locktime
		result = append(result, data)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].unixint < result[j].unixint })

	return result, nil
}
