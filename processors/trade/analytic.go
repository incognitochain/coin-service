package trade

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/analyticdb"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/transaction"
)

var analyticState State

func startAnalytic() {
	err := loadState(&analyticState, "trade-analytic")
	if err != nil {
		panic(err)
	}
	//create trade-result-table
	err = createAnalyticTable()
	if err != nil {
		panic(err)
	}

	err = createContinuousView()
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(5 * time.Second)
		startTime := time.Now()

		metas := []string{strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta), strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

		txList, err := getTxToProcess(metas, historyState.LastProcessedObjectID, 10000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		data, err := extractDataForAnalytic(txList)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		err = ingestToTimescale(data)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}

		fmt.Println("mongo time", time.Since(startTime))
		if len(txList) != 0 {
			analyticState.LastProcessedObjectID = txList[len(txList)-1].ID.Hex()
			err = updateState(&analyticState, "trade-analytic")
			if err != nil {
				panic(err)
			}
		}

		fmt.Println("process-analytic time", time.Since(startTime))
	}
}

func extractDataForAnalytic(txlist []shared.TxData) ([]AnalyticTradeData, error) {
	var result []AnalyticTradeData
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
		case metadata.PDECrossPoolTradeResponseMeta, metadata.PDETradeResponseMeta, metadata.Pdexv3TradeResponseMeta, metadata.Pdexv3AddOrderResponseMeta:
			requestTx := ""
			switch metaDataType {
			case metadata.PDECrossPoolTradeResponseMeta:
				continue
				statusStr := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
				if statusStr != "xPoolTradeAccepted" {
					continue
				}
				requestTx = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
			case metadata.PDETradeResponseMeta:
				continue
				statusStr := txDetail.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
				if statusStr != "accepted" {
					continue
				}
				requestTx = txDetail.GetMetadata().(*metadata.PDETradeResponse).RequestedTxID.String()
			case metadata.Pdexv3TradeResponseMeta:
				continue
				md := txDetail.GetMetadata().(*metadataPdexv3.TradeResponse)
				if md.Status != 0 {
					continue
				}
				requestTx = md.RequestTxID.String()
			case metadata.Pdexv3AddOrderResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.AddOrderResponse)
				if md.Status != 0 {
					continue
				}
				requestTx = md.RequestTxID.String()
			}
			// tokenIDStr := txDetail.GetTokenID().String()
			amount := uint64(0)
			if txDetail.GetType() == common.TxCustomTokenPrivacyType || txDetail.GetType() == common.TxTokenConversionType {
				txToken := txDetail.(transaction.TransactionToken)
				if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
					outs := txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
					if len(outs) > 0 {
						amount = outs[0].GetValue()
						if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
							// txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
							// tokenIDStr = txTokenData.PropertyID.String()
						}
					}
				}
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				if len(outs) > 0 {
					amount = outs[0].GetValue()
				}
			}
			poolID, rate, isbuy, err := getPoolAndRate(requestTx)
			if err != nil {
				return nil, err
			}
			trade := AnalyticTradeData{
				Time:    time.Unix(txDetail.GetLockTime(), 0),
				TradeId: requestTx,
				PairID:  poolID,
				PoolID:  poolID,
				Rate:    rate,
			}
			if isbuy {
				trade.Token1Amount = int(amount)
				trade.Token2Amount = int(float64(amount) * rate)
			} else {
				trade.Token1Amount = int(float64(amount) / rate)
				trade.Token2Amount = int(amount)
			}
			result = append(result, trade)
		case metadata.Pdexv3WithdrawOrderResponseMeta:
			// meta := txDetail.GetMetadata().(*metadataPdexv3.WithdrawOrderResponse)
			// order := shared.TradeOrderData{
			// 	WithdrawTxs:      []string{meta.RequestTxID.String()},
			// 	WithdrawPendings: []string{meta.RequestTxID.String()},
			// 	WithdrawInfos:    make(map[string]shared.TradeOrderWithdrawInfo),
			// }
			// tokenIDStr := txDetail.GetTokenID().String()
			// amount := uint64(0)
			// if txDetail.GetType() == common.TxCustomTokenPrivacyType || txDetail.GetType() == common.TxTokenConversionType {
			// 	txToken := txDetail.(transaction.TransactionToken)
			// 	if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
			// 		outs := txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
			// 		if len(outs) > 0 {
			// 			amount = outs[0].GetValue()
			// 			if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
			// 				txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
			// 				tokenIDStr = txTokenData.PropertyID.String()
			// 			}
			// 		}
			// 	}
			// } else {
			// 	outs := txDetail.GetProof().GetOutputCoins()
			// 	if len(outs) > 0 {
			// 		amount = outs[0].GetValue()
			// 	}
			// }
			// order.WithdrawInfos[meta.RequestTxID.String()] = shared.TradeOrderWithdrawInfo{
			// 	Status:        []int{meta.Status},
			// 	Responds:      []string{tx.TxHash},
			// 	RespondTokens: []string{tokenIDStr},
			// 	RespondAmount: []uint64{amount},
			// }
			// cancelRespond = append(cancelRespond, order)
		}
	}
	return result, nil
}

func createAnalyticTable() error {
	queryCreateHypertable := `CREATE TABLE IF NOT EXISTS trade_data (
		time TIMESTAMPTZ NOT NULL,
		trade_id TEXT,
		rate DOUBLE PRECISION,
		pair_id TEXT,
		pool_id TEXT,
		actual_token1_amount INTEGER,
		actual_token2_amount INTEGER
		);
		SELECT create_hypertable('trade_data', 'time');
		`
	_, err := analyticdb.Exec(context.Background(), queryCreateHypertable)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}

func ingestToTimescale(data []AnalyticTradeData) error {
	//TODO
	return nil
}

func createContinuousView() error {
	var err error
	table := "trade_result_table"
	//create view
	//15m view
	queryCreateView15m := genCAViewQuery(view15m, table, period15m)
	//30m view
	queryCreateView30m := genCAViewQuery(view30m, table, period30m)
	//1h view
	queryCreateView1h := genCAViewQuery(view1h, table, period1h)
	//4h view
	queryCreateView4h := genCAViewQuery(view4h, table, period4h)
	//1d view
	queryCreateView1d := genCAViewQuery(view1d, table, period1d)
	//1w view
	queryCreateView1w := genCAViewQuery(view1w, table, period1w)

	//create agg-policy
	//15m view
	queryContinuousAgg15m := genConAggPolicyQuery(view15m)
	//30m view
	queryContinuousAgg30m := genConAggPolicyQuery(view30m)
	//1h view
	queryContinuousAgg1h := genConAggPolicyQuery(view1h)
	//4h view
	queryContinuousAgg4h := genConAggPolicyQuery(view4h)
	//1d view
	queryContinuousAgg1d := genConAggPolicyQuery(view1d)
	//1w view
	queryContinuousAgg1w := genConAggPolicyQuery(view1w)

	//create retention-policy
	//TODO

	_, err = analyticdb.Exec(context.Background(), queryCreateView15m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView30m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView4h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1d)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryCreateView1w)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}

	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg15m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg30m)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg4h)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1d)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	_, err = analyticdb.Exec(context.Background(), queryContinuousAgg1w)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}

func getPoolAndRate(requestTx string) (string, float64, bool, error) {
	txs, err := database.DBGetTxByHash([]string{requestTx})
	if err != nil {
		return "", 0, false, err
	}
	for _, tx := range txs {
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
			// requestTx := txDetail.Hash().String()
			// lockTime := txDetail.GetLockTime()
			// buyToken := ""
			sellToken := ""
			poolID := ""
			// pairID := ""
			minaccept := uint64(0)
			amount := uint64(0)
			// nftID := ""
			// isSwap := true
			// version := 1
			// feeToken := ""
			// fee := uint64(0)
			rate := float64(0)
			isBuy := false
			// tradingPath := []string{}
			switch metaDataType {
			case metadata.PDETradeRequestMeta:
				// meta := txDetail.GetMetadata().(*metadata.PDETradeRequest)
				// buyToken = meta.TokenIDToBuyStr
				// sellToken = meta.TokenIDToSellStr
				// pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				// minaccept = meta.MinAcceptableAmount
				// amount = meta.SellAmount
			case metadata.PDECrossPoolTradeRequestMeta:
				// meta := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				// buyToken = meta.TokenIDToBuyStr
				// sellToken = meta.TokenIDToSellStr
				// pairID = meta.TokenIDToBuyStr + "-" + meta.TokenIDToSellStr
				// minaccept = meta.MinAcceptableAmount
				// amount = meta.SellAmount
			case metadata.Pdexv3TradeRequestMeta:
				// item, ok := txDetail.GetMetadata().(*metadataPdexv3.TradeRequest)
				// if !ok {
				// 	panic("invalid metadataPdexv3.TradeRequest")
				// }
				// sellToken = item.TokenToSell.String()
				// pairID = strings.Join(item.TradePath, "-")
				// if len(item.TradePath) == 1 {
				// 	tks := strings.Split(item.TradePath[0], "-")
				// 	if tks[0] == sellToken {
				// 		buyToken = tks[1]
				// 	} else {
				// 		buyToken = tks[0]
				// 	}
				// } else {
				// 	tksMap := make(map[string]bool)
				// 	for _, path := range item.TradePath {
				// 		tks := strings.Split(path, "-")
				// 		for idx, v := range tks {
				// 			if idx+1 == len(tks) {
				// 				continue
				// 			}
				// 			if _, ok := tksMap[v]; ok {
				// 				tksMap[v] = false
				// 			} else {
				// 				tksMap[v] = true
				// 			}
				// 		}

				// 	}
				// 	for k, v := range tksMap {
				// 		if v && k != sellToken {
				// 			buyToken = k
				// 		}
				// 	}
				// }

				// if sellToken == common.PRVCoinID.String() {
				// 	feeToken = common.PRVCoinID.String()
				// } else {
				// 	// error was handled by tx validation
				// 	_, burnedPRVCoin, _, _, _ := txDetail.GetTxFullBurnData()
				// 	if burnedPRVCoin == nil {
				// 		feeToken = sellToken
				// 	} else {
				// 		feeToken = common.PRVCoinID.String()
				// 	}
				// }

				// minaccept = item.MinAcceptableAmount
				// amount = item.SellAmount
				// version = 2
				// tradingPath = item.TradePath
				// fee = item.TradingFee
			case metadata.Pdexv3AddOrderRequestMeta:
				// isSwap = false
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.AddOrderRequest)
				if !ok {
					panic("invalid metadataPdexv3.AddOrderRequest")
				}
				sellToken = item.TokenToSell.String()
				poolID = item.PoolPairID
				minaccept = item.MinAcceptableAmount
				amount = item.SellAmount
				tokenStrs := strings.Split(poolID, "-")
				if sellToken != tokenStrs[0] {
					// buyToken = tokenStrs[0]
					rate = float64(minaccept / amount)
					isBuy = true
				} else {
					// buyToken = tokenStrs[1]
					rate = float64(amount / minaccept)
				}
			}
			return poolID, rate, isBuy, nil
		}
	}
	return "", 0, false, nil
}
