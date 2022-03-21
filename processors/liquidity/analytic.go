package liquidity

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/analyticdb"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/jackc/pgx"
)

var analyticState State

func startAnalytic() {
	err := loadState(&analyticState, "liquidity-analytic")
	if err != nil {
		panic(err)
	}
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
		metas := []string{strconv.Itoa(metadata.Pdexv3TradeResponseMeta), strconv.Itoa(metadata.Pdexv3AddOrderResponseMeta), strconv.Itoa(metadata.Pdexv3WithdrawOrderResponseMeta)}

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

func createAnalyticTable() error {

	return nil
}

func createContinuousView() error {

	return nil
}

func extractDataForAnalytic(txlist []shared.TxData) ([]AnalyticLiquidityData, error) {
	var result []AnalyticLiquidityData
	for _, v := range txlist {

	}
	return result, nil
}

func ingestToTimescale(data []AnalyticLiquidityData) error {
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
