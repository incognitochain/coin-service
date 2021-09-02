package shield

import (
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var currentState State

func StartProcessor() {

	err := database.DBCreateShieldIndex()
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

		request, respond, err := processShieldTxs(txList)
		if err != nil {
			panic(err)
		}
		_ = request
		err = database.DBSaveTxShield(respond)
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
	if result == nil {
		currentState = State{}
		return nil
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

func processShieldTxs(shieldTxs []shared.TxData) ([]shared.ShieldData, []shared.ShieldData, error) {
	var requestData []shared.ShieldData

	var respondData []shared.ShieldData
	for _, tx := range shieldTxs {
		metaDataType, _ := strconv.Atoi(tx.Metatype)
		requestTx := ""
		bridge := ""
		isDecentralized := false
		if metaDataType == metadata.IssuingResponseMeta || metaDataType == metadata.IssuingETHResponseMeta || metaDataType == metadata.IssuingBSCResponseMeta {
			switch metaDataType {
			case metadata.IssuingResponseMeta:
				meta := metadata.IssuingResponse{}
				json.Unmarshal([]byte(tx.Metadata), &meta)
				requestTx = meta.RequestedTxID.String()
				bridge = "btc"
			case metadata.IssuingETHResponseMeta, metadata.IssuingBSCResponseMeta:
				meta := metadata.IssuingEVMResponse{}
				json.Unmarshal([]byte(tx.Metadata), &meta)
				requestTx = meta.RequestedTxID.String()
				if metaDataType == metadata.IssuingETHResponseMeta {
					bridge = "eth"
				} else {
					bridge = "bsc"
				}
			}
			txChoice, parseErr := shared.DeserializeTransactionJSON([]byte(tx.TxDetail))
			if parseErr != nil {
				panic(parseErr)
			}
			txDetail := txChoice.ToTx()
			if txDetail == nil {
				panic(errors.New("invalid tx detected"))
			}
			tokenIDStr := txDetail.GetTokenID().String()
			amount := uint64(0)
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
			}
			shieldData := shared.NewShieldData(requestTx, tx.TxHash, tokenIDStr, bridge, "", isDecentralized, amount, tx.BlockHeight)
			respondData = append(respondData, *shieldData)
		}

	}
	return requestData, respondData, nil
}
