package trade

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		time.Sleep(10 * time.Second)

		txList, err := getTxToProcess(currentState.LastProcessedObjectID, 1000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		request, respond, withdrawReq, withdrawRes, err := processTradeToken(txList)
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
		err = database.DBUpdateWithdrawTradeOrderReq(withdrawReq)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateWithdrawTradeOrderRes(withdrawRes)
		if err != nil {
			panic(err)
		}

		if len(txList) != 0 {
			currentState.LastProcessedObjectID = txList[len(txList)-1].ID.Hex()
			err = updateState()
			if err != nil {
				panic(err)
			}
		}
		err = updateTradeStatus()
		if err != nil {
			panic(err)
		}
	}
}

func getTxToProcess(lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta), strconv.Itoa(metadata.PDETradeRequestMeta), strconv.Itoa(metadata.Pdexv3TradeRequestMeta), strconv.Itoa(metadata.Pdexv3AddOrderRequestMeta), strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta), strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderRequestMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}
	var obID primitive.ObjectID
	if lastID == "" {
		obID = primitive.ObjectID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	} else {
		var err error
		obID, err = primitive.ObjectIDFromHex(lastID)
		if err != nil {
			return nil, err
		}
	}
	filter := bson.M{
		"_id":      bson.M{operator.Gt: obID},
		"metatype": bson.M{operator.In: metas},
	}
	err := mgm.Coll(&shared.TxData{}).SimpleFind(&result, filter, &options.FindOptions{
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
	return database.DBUpdateProcessorState("trade", string(result))
}

func loadState() error {
	result, err := database.DBGetProcessorState("trade")
	if err != nil {
		return err
	}
	if result == nil {
		currentState = State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}

func processTradeToken(txlist []shared.TxData) ([]shared.TradeOrderData, []shared.TradeOrderData, []shared.TradeOrderData, []shared.TradeOrderData, error) {
	var requestTrades []shared.TradeOrderData
	var respondTrades []shared.TradeOrderData
	var cancelTrades []shared.TradeOrderData
	var cancelRespond []shared.TradeOrderData
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
		// txDetail.GetTxFullBurnData()
		switch metaDataType {
		case metadata.PDECrossPoolTradeRequestMeta, metadata.PDETradeRequestMeta, metadata.Pdexv3TradeRequestMeta, metadata.Pdexv3AddOrderRequestMeta:
			requestTx := txDetail.Hash().String()
			lockTime := txDetail.GetLockTime()
			buyToken := ""
			sellToken := ""
			poolID := ""
			pairID := ""
			minaccept := uint64(0)
			amount := uint64(0)
			nftID := ""
			isSwap := true
			version := 1
			feeToken := ""
			fee := uint64(0)
			tradingPath := []string{}
			switch metaDataType {
			case metadata.PDETradeRequestMeta:
				meta := txDetail.GetMetadata().(*metadata.PDETradeRequest)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				minaccept = meta.MinAcceptableAmount
				amount = meta.SellAmount
			case metadata.PDECrossPoolTradeRequestMeta:
				meta := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				buyToken = meta.TokenIDToBuyStr
				sellToken = meta.TokenIDToSellStr
				pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				minaccept = meta.MinAcceptableAmount
				amount = meta.SellAmount
			case metadata.Pdexv3TradeRequestMeta:
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.TradeRequest)
				if !ok {
					panic("invalid metadataPdexv3.TradeRequest")
				}
				sellToken = item.TokenToSell.String()
				pairID = strings.Join(item.TradePath, "-")
				if len(item.TradePath) == 1 {
					tks := strings.Split(item.TradePath[0], "-")
					if tks[0] == sellToken {
						buyToken = tks[1]
					} else {
						buyToken = tks[0]
					}
				} else {
					tksMap := make(map[string]bool)
					for _, path := range item.TradePath {
						tks := strings.Split(path, "-")
						for idx, v := range tks {
							if idx+1 == len(tks) {
								continue
							}
							if _, ok := tksMap[v]; ok {
								tksMap[v] = false
							} else {
								tksMap[v] = true
							}
						}

					}
					for k, v := range tksMap {
						if v && k != sellToken {
							buyToken = k
						}
					}
				}

				if sellToken == common.PRVCoinID.String() {
					feeToken = common.PRVCoinID.String()
				} else {
					// error was handled by tx validation
					_, burnedPRVCoin, _, _, _ := txDetail.GetTxFullBurnData()
					if burnedPRVCoin == nil {
						feeToken = sellToken
					} else {
						feeToken = common.PRVCoinID.String()
					}
				}

				minaccept = item.MinAcceptableAmount
				amount = item.SellAmount
				version = 2
				tradingPath = item.TradePath
				fee = item.TradingFee
			case metadata.Pdexv3AddOrderRequestMeta:
				isSwap = false
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.AddOrderRequest)
				if !ok {
					panic("invalid metadataPdexv3.AddOrderRequest")
				}
				sellToken = item.TokenToSell.String()
				poolID = item.PoolPairID
				tokenStrs := strings.Split(poolID, "-")
				if sellToken != tokenStrs[0] {
					buyToken = tokenStrs[0]
				} else {
					buyToken = tokenStrs[1]
				}
				pairID = tokenStrs[0] + "-" + tokenStrs[1]
				minaccept = item.MinAcceptableAmount
				amount = item.SellAmount
				nftID = item.NftID.String()
				version = 2

			}

			trade := shared.NewTradeOrderData(requestTx, sellToken, buyToken, poolID, pairID, nftID, 0, fmt.Sprintf("%v", minaccept), fmt.Sprintf("%v", amount), lockTime, tx.ShardID, tx.BlockHeight)
			trade.Version = version
			trade.IsSwap = isSwap
			trade.TradingPath = tradingPath
			trade.Fee = fee
			trade.FeeToken = feeToken
			requestTrades = append(requestTrades, *trade)
		case metadata.PDECrossPoolTradeResponseMeta, metadata.PDETradeResponseMeta, metadata.Pdexv3TradeResponseMeta, metadata.Pdexv3AddOrderResponseMeta:
			status := 0
			requestTx := ""
			switch metaDataType {
			case metadata.PDECrossPoolTradeResponseMeta:
				statusStr := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
				if statusStr == "xPoolTradeAccepted" {
					status = 1
				} else {
					status = 2
				}
				requestTx = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
			case metadata.PDETradeResponseMeta:
				statusStr := txDetail.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
				if statusStr == "accepted" {
					status = 1
				} else {
					status = 2
				}
				requestTx = txDetail.GetMetadata().(*metadata.PDETradeResponse).RequestedTxID.String()
			case metadata.Pdexv3TradeResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.TradeResponse)
				if md.Status == 0 {
					status = 2
				} else {
					status = 1
				}
				requestTx = md.RequestTxID.String()
			case metadata.Pdexv3AddOrderResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.AddOrderResponse)
				if md.Status == 0 {
					status = 2
				} else {
					status = 1
				}
				requestTx = md.RequestTxID.String()
			}
			tokenIDStr := txDetail.GetTokenID().String()
			amount := uint64(0)
			if txDetail.GetType() == common.TxCustomTokenPrivacyType || txDetail.GetType() == common.TxTokenConversionType {
				txToken := txDetail.(transaction.TransactionToken)
				if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
					outs := txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
					if len(outs) > 0 {
						amount = outs[0].GetValue()
						if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
							txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
							tokenIDStr = txTokenData.PropertyID.String()
						}
					}
				}
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				if len(outs) > 0 {
					amount = outs[0].GetValue()
				}
			}
			trade := shared.TradeOrderData{
				RequestTx:     requestTx,
				Status:        status,
				RespondTxs:    []string{txDetail.Hash().String()},
				RespondTokens: []string{tokenIDStr},
				RespondAmount: []uint64{amount},
			}
			respondTrades = append(respondTrades, trade)
		case metadata.Pdexv3WithdrawOrderRequestMeta:
			meta := txDetail.GetMetadata().(*metadataPdexv3.WithdrawOrderRequest)

			wdData := shared.TradeOrderWithdrawInfo{
				TokenIDs:      []string{},
				Status:        []int{},
				Responds:      []string{},
				RespondTokens: []string{},
				RespondAmount: []uint64{},
			}
			for tokenID, _ := range meta.Receiver {
				wdData.TokenIDs = append(wdData.TokenIDs, tokenID.String())
			}
			wdData.Amount = meta.Amount
			order := shared.TradeOrderData{
				RequestTx:        meta.OrderID,
				WithdrawTxs:      []string{tx.TxHash},
				WithdrawPendings: []string{tx.TxHash},
				WithdrawInfos:    make(map[string]shared.TradeOrderWithdrawInfo),
				NFTID:            meta.NftID.String(),
			}
			order.WithdrawInfos[tx.TxHash] = wdData
			cancelTrades = append(cancelTrades, order)
		case metadata.Pdexv3WithdrawOrderResponseMeta:
			meta := txDetail.GetMetadata().(*metadataPdexv3.WithdrawOrderResponse)
			order := shared.TradeOrderData{
				WithdrawTxs:      []string{meta.RequestTxID.String()},
				WithdrawPendings: []string{meta.RequestTxID.String()},
				WithdrawInfos:    make(map[string]shared.TradeOrderWithdrawInfo),
			}
			tokenIDStr := txDetail.GetTokenID().String()
			amount := uint64(0)
			if txDetail.GetType() == common.TxCustomTokenPrivacyType || txDetail.GetType() == common.TxTokenConversionType {
				txToken := txDetail.(transaction.TransactionToken)
				if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
					outs := txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
					if len(outs) > 0 {
						amount = outs[0].GetValue()
						if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
							txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
							tokenIDStr = txTokenData.PropertyID.String()
						}
					}
				}
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				if len(outs) > 0 {
					amount = outs[0].GetValue()
				}
			}
			order.WithdrawInfos[meta.RequestTxID.String()] = shared.TradeOrderWithdrawInfo{
				Status:        []int{meta.Status},
				Responds:      []string{tx.TxHash},
				RespondTokens: []string{tokenIDStr},
				RespondAmount: []uint64{amount},
			}
			cancelRespond = append(cancelRespond, order)
		}
	}
	return requestTrades, respondTrades, cancelTrades, cancelRespond, nil
}

func updateTradeStatus() error {
	limit := int64(10000)
	offset := int64(0)

	for {
		list, err := database.DBGetPendingWithdrawOrder(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.TradeOrderData{}
		for _, v := range list {
			data := shared.TradeOrderData{
				RequestTx:        v.RequestTx,
				WithdrawInfos:    v.WithdrawInfos,
				WithdrawPendings: []string{},
			}

			for _, wdtx := range v.WithdrawTxs {
				a := v.WithdrawInfos[wdtx]
				i, err := database.DBGetBeaconInstructionByTx(wdtx)
				if i == nil && err == nil {
					fmt.Println("wdtx", wdtx)
					continue
				}
				if err != nil {
					panic(err)
				}
				if i.Status == "0" {
					a.IsRejected = true
					data.WithdrawInfos[wdtx] = a
					data.WithdrawPendings = append(data.WithdrawPendings, wdtx)
					listToUpdate = append(listToUpdate, data)
				}
			}
		}
		err = database.DBUpdatePDETradeWithdrawStatus(listToUpdate)
		if err != nil {
			return err
		}
	}
	return nil
}
