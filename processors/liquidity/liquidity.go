package liquidity

import (
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var currentState State

func StartProcessor() {
	err := database.DBCreateLiquidityIndex()
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
		addLiquidityTxs := []shared.TxData{}
		withdrawLiquidityTxs := []shared.TxData{}
		for _, tx := range txList {
			metaType, _ := strconv.Atoi(tx.Metatype)
			switch metaType {
			case metadataCommon.Pdexv3AddLiquidityRequestMeta:
				addLiquidityTxs = append(addLiquidityTxs, tx)
			case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
				withdrawLiquidityTxs = append(withdrawLiquidityTxs, tx)
			}
		}

		err = updateState()
		if err != nil {
			panic(err)
		}
	}
}

func getTxToProcess(lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	metas := []string{strconv.Itoa(metadataCommon.Pdexv3AddLiquidityRequestMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLiquidityRequestMeta)}
	filter := bson.M{
		"_id":      bson.M{operator.Gt: lastID},
		"metatype": bson.M{operator.In: metas},
	}
	err := mgm.Coll(&shared.TxData{}).SimpleFind(result, filter, &options.FindOptions{
		Sort:  bson.D{{"locktime", 1}},
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
	return json.UnmarshalFromString(result.State, &currentState)
}

func processAddLiquidity(txList []shared.TxData) ([]shared.ContributionData, []shared.WaitingContributions, error) {
	var contributeDatas []shared.ContributionData
	var waitingDatas []shared.WaitingContributions

	return contributeDatas, waitingDatas, nil
}

func processWithdrawLiquidity(txList []shared.TxData) error {
	return nil
}
