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
	"github.com/jackc/pgx/v4"
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

		// strconv.Itoa(metadata.PDECrossPoolTradeResponseMeta), strconv.Itoa(metadata.PDETradeResponseMeta),
		metas := []string{strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

		txList, err := getTxToProcess(metas, analyticState.LastProcessedObjectID, 10000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		data, err := extractDataForAnalytic(txList)
		if err != nil {
			log.Println("extractDataForAnalytic", err)
			continue
		}
		err = ingestToTimescale(data)
		if err != nil {
			log.Println("ingestToTimescale", err)
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
			// case metadata.PDECrossPoolTradeResponseMeta:
			// 	continue
			// statusStr := txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
			// if statusStr != "xPoolTradeAccepted" {
			// 	continue
			// }
			// requestTx = txDetail.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
			// case metadata.PDETradeResponseMeta:
			// 	continue
			// statusStr := txDetail.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
			// if statusStr != "accepted" {
			// 	continue
			// }
			// requestTx = txDetail.GetMetadata().(*metadata.PDETradeResponse).RequestedTxID.String()
			case metadata.Pdexv3TradeResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.TradeResponse)
				if md.Status != metadataPdexv3.TradeAcceptedStatus {
					continue
				}
				requestTx = md.RequestTxID.String()
			case metadata.Pdexv3AddOrderResponseMeta:
				md := txDetail.GetMetadata().(*metadataPdexv3.AddOrderResponse)
				if md.Status != metadataPdexv3.OrderAcceptedStatus {
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
						for _, v := range outs {
							if !v.IsEncrypted() {
								amount += v.GetValue()
							}
						}
						// if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
						// txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
						// tokenIDStr = txTokenData.PropertyID.String()
						// }
					}
				}
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				if len(outs) > 0 {
					for _, v := range outs {
						if !v.IsEncrypted() {
							amount += v.GetValue()
						}
					}
				}
			}
			tradeInfo, err := getPoolAndRate(requestTx)
			if err != nil {
				return nil, err
			}
			trade := AnalyticTradeData{
				Time:       time.Unix(txDetail.GetLockTime(), 0),
				TradeId:    requestTx,
				PairID:     tradeInfo.PairID,
				SellPoolID: tradeInfo.TradePath[0],
				BuyPoolID:  tradeInfo.TradePath[len(tradeInfo.TradePath)-1],
				Rate:       tradeInfo.Rate,
			}
			if tradeInfo.IsSwap {
				trade.Token1Amount = tradeInfo.SellAmount
				trade.Token2Amount = amount
			} else {
				if tradeInfo.IsBuy {
					trade.Token1Amount = tradeInfo.BuyAmount
					trade.Token2Amount = tradeInfo.SellAmount
				} else {
					trade.Token1Amount = tradeInfo.SellAmount
					trade.Token2Amount = tradeInfo.BuyAmount
				}
			}

			result = append(result, trade)
		case metadata.Pdexv3WithdrawOrderResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawOrderResponse)
			if md.Status != metadataPdexv3.WithdrawOrderAcceptedStatus {
				continue
			}
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
		sell_pool_id TEXT,
		buy_pool_id TEXT,
		actual_token1_amount bigint,
		actual_token2_amount bigint
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
	queryInsertTimeseriesData := `
	INSERT INTO trade_data (time, trade_id, rate, pair_id, sell_pool_id, buy_pool_id, actual_token1_amount, actual_token2_amount) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);`

	batch := &pgx.Batch{}
	for _, r := range data {
		batch.Queue(queryInsertTimeseriesData, r.Time, r.TradeId, r.Rate, r.PairID, r.SellPoolID, r.BuyPoolID, r.Token1Amount, r.Token2Amount)
	}
	_, err := analyticdb.ExecBatch(context.Background(), batch)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}

func createContinuousView() error {
	var err error
	table := "trade_data"
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

func getPoolAndRate(requestTx string) (*tradeInfo, error) {
	var result tradeInfo
	txs, err := database.DBGetTxByHash([]string{requestTx})
	if err != nil {
		return nil, err
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
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.TradeRequest)
				if !ok {
					panic("invalid metadataPdexv3.TradeRequest")
				}
				result.TokenSell = item.TokenToSell.String()
				result.TradePath = item.TradePath
				if len(item.TradePath) == 1 {
					tks := strings.Split(item.TradePath[0], "-")
					if tks[0] == result.TokenSell {
						result.TokenBuy = tks[1]
					} else {
						result.TokenBuy = tks[0]
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
						if v && k != result.TokenSell {
							result.TokenBuy = k
						}
					}
				}

				// if result.TokenSell == common.PRVCoinID.String() {
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

				result.BuyAmount = item.MinAcceptableAmount
				result.SellAmount = item.SellAmount
				result.PairID = shared.GenPairID(result.TokenSell, result.TokenBuy)
				result.Rate = float64(item.MinAcceptableAmount) / float64(item.SellAmount)
			case metadata.Pdexv3AddOrderRequestMeta:
				item, ok := txDetail.GetMetadata().(*metadataPdexv3.AddOrderRequest)
				if !ok {
					panic("invalid metadataPdexv3.AddOrderRequest")
				}
				minaccept := item.MinAcceptableAmount
				amount := item.SellAmount
				result.IsSwap = false
				result.TokenSell = item.TokenToSell.String()
				result.TradePath = []string{item.PoolPairID}
				result.BuyAmount = minaccept
				result.SellAmount = amount
				tokenStrs := strings.Split(item.PoolPairID, "-")
				if result.TokenSell != tokenStrs[0] {
					result.Rate = float64(minaccept / amount)
					result.IsBuy = true
				} else {
					result.Rate = float64(amount / minaccept)
				}
				result.PairID = shared.GenPairID(result.TokenSell, result.TokenBuy)
			}
			return &result, nil
		}
	}
	return nil, errors.New("???")
}
