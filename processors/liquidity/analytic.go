package liquidity

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
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/jackc/pgx/v4"
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

		metas := []string{strconv.Itoa(metadataCommon.Pdexv3AddLiquidityRequestMeta), strconv.Itoa(metadataCommon.Pdexv3AddLiquidityResponseMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLiquidityResponseMeta), strconv.Itoa(metadataCommon.Pdexv3StakingRequestMeta), strconv.Itoa(metadataCommon.Pdexv3StakingResponseMeta), strconv.Itoa(metadataCommon.Pdexv3UnstakingResponseMeta)}

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
			err = updateState(&analyticState, "liquidity-analytic")
			if err != nil {
				panic(err)
			}
		}
		fmt.Println("process-analytic time", time.Since(startTime))
	}
}

func createAnalyticTable() error {
	queryCreateHypertable := `CREATE TABLE IF NOT EXISTS liquidity_data (
		time TIMESTAMPTZ NOT NULL,
		id TEXT,
		pool_id TEXT,
		actual_token1_amount bigint,
		actual_token2_amount bigint
		);
		SELECT create_hypertable('liquidity_data', 'time');
		`
	_, err := analyticdb.Exec(context.Background(), queryCreateHypertable)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}

func extractDataForAnalytic(txlist []shared.TxData) ([]AnalyticLiquidityData, error) {
	var result []AnalyticLiquidityData
	for _, tx := range txlist {
		data := AnalyticLiquidityData{
			ID: tx.TxHash,
		}
		metaDataType, _ := strconv.Atoi(tx.Metatype)
		txChoice, parseErr := shared.DeserializeTransactionJSON([]byte(tx.TxDetail))
		if parseErr != nil {
			panic(parseErr)
		}
		txDetail := txChoice.ToTx()
		if txDetail == nil {
			panic(errors.New("invalid tx detected"))
		}
		data.Time = time.Unix(txDetail.GetLockTime(), 0)

		switch metaDataType {
		case metadataCommon.Pdexv3AddLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.AddLiquidityRequest)
			data.PoolID = md.PoolPairID()
			if data.PoolID == "" {
				continue
			} else {

			}
			tokens := strings.Split(data.PoolID, "-")
			if len(tokens) > 2 {
				if md.TokenID() == tokens[0] {
					data.Token1Amount = int64(md.TokenAmount())
				}
				if md.TokenID() == tokens[1] {
					data.Token2Amount = int64(md.TokenAmount())
				}
			}

		case metadataCommon.Pdexv3AddLiquidityResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.AddLiquidityResponse)
			requestTx := md.TxReqID()
			tokenIDStr := txDetail.GetTokenID().String()
			tokenIDStr2, amount := extractTxOutputAmount(txDetail)
			if tokenIDStr2 != "" {
				tokenIDStr = tokenIDStr2
			}
			poolID, err := retrievePoolIDFromRequestTx(requestTx)
			if err != nil {
				panic(err)
			}
			data.PoolID = poolID
			tokens := strings.Split(data.PoolID, "-")
			if len(tokens) > 2 {
				if tokenIDStr == tokens[0] {
					data.Token1Amount = -int64(amount)
				}
				if tokenIDStr == tokens[1] {
					data.Token2Amount = -int64(amount)
				}
			}
		case metadataCommon.Pdexv3WithdrawLiquidityResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityResponse)
			tokenIDStr := txDetail.GetTokenID().String()
			tokenIDStr2, amount := extractTxOutputAmount(txDetail)
			if tokenIDStr2 != "" {
				tokenIDStr = tokenIDStr2
			}
			poolID, err := retrievePoolIDFromRequestTx(md.TxReqID())
			if err != nil {
				panic(err)
			}
			data.PoolID = poolID
			tokens := strings.Split(data.PoolID, "-")
			if tokenIDStr == tokens[0] {
				data.Token1Amount = -int64(amount)
			}
			if tokenIDStr == tokens[1] {
				data.Token2Amount = -int64(amount)
			}
		case metadataCommon.Pdexv3StakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingRequest)
			data.PoolID = md.TokenID()
			data.Token1Amount = int64(md.TokenAmount())
		case metadataCommon.Pdexv3StakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingResponse)
			if md.Status() == common.Pdexv3AcceptStringStatus {
				continue
			} else {
				poolID, err := retrievePoolIDFromRequestTx(md.TxReqID())
				if err != nil {
					panic(err)
				}
				data.PoolID = poolID
				_, amount := extractTxOutputAmount(txDetail)
				data.Token1Amount = -int64(amount)
			}
		case metadataCommon.Pdexv3UnstakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.UnstakingResponse)
			if md.Status() == common.Pdexv3AcceptStringStatus {
				poolID, err := retrievePoolIDFromRequestTx(md.TxReqID())
				if err != nil {
					panic(err)
				}
				data.PoolID = poolID
				_, amount := extractTxOutputAmount(txDetail)
				data.Token1Amount = -int64(amount)
			}
		}
		result = append(result, data)
	}
	return result, nil
}

func ingestToTimescale(data []AnalyticLiquidityData) error {
	queryInsertTimeseriesData := `
	INSERT INTO liquidity_data (time, id, pool_id, actual_token1_amount, actual_token2_amount) VALUES ($1, $2, $3, $4, $5);`

	batch := &pgx.Batch{}
	for _, r := range data {
		batch.Queue(queryInsertTimeseriesData, r.Time, r.ID, r.PoolID, r.Token1Amount, r.Token2Amount)
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
	queryCAView := genCAViewQuery("contribute_total", "liquidity_data", "1 days")

	queryCAPolicy := genConAggPolicyQuery("contribute_total")

	_, err = analyticdb.Exec(context.Background(), queryCAView)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}

	_, err = analyticdb.Exec(context.Background(), queryCAPolicy)
	if err != nil {
		if !analyticdb.IsAlreadyExistError(err.Error()) {
			return err
		}
	}
	return nil
}

func retrievePoolIDFromRequestTx(requestTx string) (string, error) {
	var poolid string
	txs, err := database.DBGetTxByHash([]string{requestTx})
	if err != nil {
		return poolid, err
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
		case metadataCommon.Pdexv3AddLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.AddLiquidityRequest)
			poolid = md.PoolPairID()
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityRequest)
			poolid = md.PoolPairID()

		case metadataCommon.Pdexv3StakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingRequest)
			poolid = md.TokenID()

		case metadataCommon.Pdexv3UnstakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.UnstakingRequest)
			poolid = md.StakingPoolID()

		}
	}

	return poolid, nil
}

func extractTxOutputAmount(txDetail metadataCommon.Transaction) (string, uint64) {
	amount := uint64(0)
	tokenIDStr := ""
	if txDetail.GetType() == common.TxCustomTokenPrivacyType || txDetail.GetType() == common.TxTokenConversionType {
		txToken := txDetail.(transaction.TransactionToken)
		if txToken.GetTxTokenData().TxNormal.GetProof() != nil {
			outs := txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
			amount = outs[0].GetValue()
			if outs[0].GetVersion() == 2 && !txDetail.IsPrivacy() {
				txTokenData := transaction.GetTxTokenDataFromTransaction(txDetail)
				tokenIDStr = txTokenData.PropertyID.String()
			}
		}
	} else {
		outs := txDetail.GetProof().GetOutputCoins()
		amount = outs[0].GetValue()
	}
	return tokenIDStr, amount
}
