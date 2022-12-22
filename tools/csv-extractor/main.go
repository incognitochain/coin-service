package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/shared"
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

	var swapList []TradeCSV
	var orderList []TradeCSV
	var list []TradeCSV

	offset := int64(0)
	for {
		list, err := getTradeCSVSwap(fromTime, toTime, offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			break
		}
		swapList = append(swapList, list...)
		fmt.Println("processed", len(list))
		offset += 40000
	}

	offset = int64(0)
	for {
		list, err := getTradeCSVOrder(fromTime, toTime, offset)
		if err != nil {
			panic(err)
		}
		if list == nil {
			break
		}
		orderList = append(orderList, list...)
		fmt.Println("processed", len(list))
		offset += 40000
	}

	list = append(list, swapList...)
	list = append(list, orderList...)
	sort.Slice(list, func(i, j int) bool { return list[i].unixint < list[j].unixint })

	for _, v := range list {
		v.CSVrow(csvTradeFile)
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

func getTradeCSVSwap(fromTime, toTime, offset int64) ([]TradeCSV, error) {
	limit := int64(40000)
	var result []TradeCSV
	var tradeSwapList []shared.TradeOrderData

	AllowDiskUse := true

	filter := bson.M{"version": bson.M{operator.Eq: 2}, "requesttime": bson.M{operator.Gte: fromTime, operator.Lt: toTime}, "isswap": bson.M{operator.Eq: true}, "status": bson.M{operator.Eq: 1}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&tradeSwapList, filter, &options.FindOptions{
		Sort:         bson.D{{"requesttime", 1}},
		AllowDiskUse: &AllowDiskUse,
		Limit:        &limit,
		Skip:         &offset,
	})
	if err != nil {
		return nil, err
	}

	for _, tradeInfo := range tradeSwapList {
		var data TradeCSV
		amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
		data.BuyToken = tradeInfo.BuyTokenID
		data.SellToken = tradeInfo.SellTokenID
		data.Amount = amount
		data.MinAcceptableReceiver = minAccept
		data.Receive = tradeInfo.RespondAmount[0]
		data.TradingFee = tradeInfo.Fee
		data.FeeToken = tradeInfo.FeeToken
		data.TxRequest = tradeInfo.RequestTx
		data.TxRespond = tradeInfo.RespondTxs[0]
		timeText := time.Unix(tradeInfo.Requesttime, 0).String()
		data.FormatedDate = timeText
		data.Unix = fmt.Sprintf("%v", tradeInfo.Requesttime)
		data.unixint = tradeInfo.Requesttime
		data.TradePath = tradeInfo.TradingPath
		result = append(result, data)
	}

	return result, nil
}

func getTradeCSVOrder(fromTime, toTime, offset int64) ([]TradeCSV, error) {
	limit := int64(40000)
	var result []TradeCSV
	var tradeOrderList []shared.TradeOrderData

	AllowDiskUse := true

	filter := bson.M{"version": bson.M{operator.Eq: 2}, "requesttime": bson.M{operator.Gte: fromTime, operator.Lt: toTime}, "isswap": bson.M{operator.Eq: false}}
	err := mgm.Coll(&shared.TradeOrderData{}).SimpleFind(&tradeOrderList, filter, &options.FindOptions{
		Sort:         bson.D{{"requesttime", 1}},
		AllowDiskUse: &AllowDiskUse,
		Limit:        &limit,
		Skip:         &offset,
	})
	if err != nil {
		return nil, err
	}
	for _, tradeInfo := range tradeOrderList {
		var data TradeCSV
		amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
		data.BuyToken = tradeInfo.BuyTokenID
		data.SellToken = tradeInfo.SellTokenID
		data.Amount = amount
		data.MinAcceptableReceiver = minAccept
		data.TradingFee = tradeInfo.Fee
		data.FeeToken = tradeInfo.FeeToken
		data.TxRequest = tradeInfo.RequestTx
		// data.TxRespond = tradeInfo.RespondTxs
		timeText := time.Unix(tradeInfo.Requesttime, 0).String()
		data.FormatedDate = timeText
		data.Unix = fmt.Sprintf("%v", tradeInfo.Requesttime)
		data.unixint = tradeInfo.Requesttime
		if len(tradeInfo.WithdrawPendings) == 0 && len(tradeInfo.WithdrawInfos) > 0 {
			for _, wdi := range tradeInfo.WithdrawInfos {
				for idx, rpTk := range wdi.RespondTokens {
					if rpTk == data.BuyToken {
						data.Receive += wdi.RespondAmount[idx]
					}
				}
			}
			if data.Receive > 0 {
				result = append(result, data)
			}
		}

	}

	return result, nil
}

// ./extractcsv -mongo='mongodb://root:example@0.0.0.0:8041' -from=1642076814 -to=1642676814
