package trade

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
)

var currentState State

func StartProcessor() {
	err := database.DBCreateTradeIndex()
	if err != nil {
		panic(err)
	}
	err = loadState()
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(5 * time.Second)

		txList, err := getTxToProcess(currentState.LastProcessedObjectID, 100)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		request, respond, cancel, err := processTradeToken(txList)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveTradeOrder(request)
		if err != nil {
			panic(err)
		}
		err = database.DBUpdateTradeOrder(respond)
		if err != nil {
			panic(err)
		}
		err = database.DBUpdateCancelTradeOrder(cancel)
		if err != nil {
			panic(err)
		}
		currentState.LastProcessedObjectID = txList[len(txList)-1].ID.String()
		err = updateState()
		if err != nil {
			panic(err)
		}
	}
}

func getTxToProcess(lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	metas := []string{}
	filter := bson.M{
		"_id":      bson.M{operator.Gt: lastID},
		"metatype": bson.M{operator.In: metas},
	}
	err := mgm.Coll(&shared.TxData{}).SimpleFind(result, filter, &options.FindOptions{
		Sort:  bson.D{{"_id", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func updateState() error {
	result, err := json.Marshal(currentState)
	if err != nil {
		panic(err)
	}
	return database.DBUpdateProcessorState("liquidity", string(result))
}

func loadState() error {
	result, err := database.DBGetProcessorState("liquidity")
	if err != nil {
		return err
	}
	if result == nil {
		currentState = State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}

func processTradeToken(txlist []shared.TxData) ([]shared.TradeOrderData, []shared.TradeOrderData, []shared.TradeOrderData, error) {
	var requestTrades []shared.TradeOrderData
	var respondTrades []shared.TradeOrderData
	var cancelTrades []shared.TradeOrderData
	for _, tx := range txlist {
		metaDataType, _ := strconv.Atoi(tx.Metatype)
		txChoice, parseErr := shared.DeserializeTransactionJSON([]byte(tx.TxDetail))
		if parseErr != nil {
			panic(parseErr)
		}
		txDetail := txChoice.ToTx()
		if txDetail == nil {
			panic(errors.New("invalid tx detected"))
		}
		switch metaDataType {
		case metadata.PDECrossPoolTradeRequestMeta, metadata.PDETradeRequestMeta, metadata.Pdexv3TradeRequestMeta, metadata.Pdexv3AddOrderRequestMeta:
			requestTx := txDetail.Hash().String()
			lockTime := txDetail.GetLockTime()
			buyToken := ""
			sellToken := ""
			poolID := ""
			pairID := ""
			rate := uint64(0)
			amount := uint64(0)
			nftID := ""
			switch metaDataType {
			case metadata.PDETradeRequestMeta:
				meta := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				rate = meta.MinAcceptableAmount / meta.SellAmount
				amount = meta.SellAmount
			case metadata.PDECrossPoolTradeRequestMeta:
				meta := txDetail.GetMetadata().(*metadata.PDETradeRequest)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				rate = meta.MinAcceptableAmount / meta.SellAmount
				amount = meta.SellAmount
			case metadata.Pdexv3TradeRequestMeta:
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.TradeRequest)
				if !ok {
					panic("invalid metadataPdexv3.TradeRequest")
				}
				sellToken = item.TokenToSell.String()
				pairID = strings.Join(item.TradePath, "-")
				rate = item.MinAcceptableAmount / item.SellAmount
				amount = item.SellAmount
			case metadata.Pdexv3AddOrderRequestMeta:
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.AddOrderRequest)
				if !ok {
					panic("invalid metadataPdexv3.AddOrderRequest")
				}
				sellToken = item.TokenToSell.String()
				poolID = item.PoolPairID
				rate = item.MinAcceptableAmount / item.SellAmount
				amount = item.SellAmount
				nftID = item.NftID.String()
			}
			trade := shared.NewTradeOrderData(requestTx, sellToken, buyToken, poolID, pairID, "", nftID, nil, rate, amount, lockTime, tx.ShardID, tx.BlockHeight)
			requestTrades = append(requestTrades, *trade)
		case metadata.PDECrossPoolTradeResponseMeta, metadata.PDETradeResponseMeta, metadata.Pdexv3TradeResponseMeta, metadata.Pdexv3AddOrderResponseMeta:
			status := ""
			requestTx := ""
			switch metaDataType {
			case metadata.PDECrossPoolTradeResponseMeta:
				status = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
				requestTx = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
			case metadata.PDETradeResponseMeta:
				status = txDetail.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
				requestTx = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
			case metadata.Pdexv3TradeResponseMeta, metadata.Pdexv3AddOrderResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.AddOrderResponse)
				if md.Status == 1 {
					status = "accepted"
				} else {
					status = "refunded"
				}

				requestTx = md.RequestTxID.String()
			}
			trade := shared.TradeOrderData{
				RequestTx:  requestTx,
				Status:     status,
				RespondTxs: []string{txDetail.Hash().String()},
			}
			respondTrades = append(respondTrades, trade)
		case metadata.Pdexv3WithdrawOrderRequestMeta:
			//TODO
		case metadata.Pdexv3WithdrawOrderResponseMeta:
			//TODO
		}
	}
	return requestTrades, respondTrades, cancelTrades, nil
}
