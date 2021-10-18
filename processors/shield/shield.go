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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		time.Sleep(10 * time.Second)

		txList, err := getTxToProcess(currentState.LastProcessedObjectID, 1000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}

		request, respond, err := processShieldTxs(txList)
		if err != nil {
			panic(err)
		}
		err = database.DBSaveTxShield(request)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateShieldData(respond)
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

	}
}

func getTxToProcess(lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	metas := []string{strconv.Itoa(metadata.IssuingRequestMeta), strconv.Itoa(metadata.IssuingBSCRequestMeta), strconv.Itoa(metadata.IssuingETHRequestMeta), strconv.Itoa(metadata.IssuingResponseMeta), strconv.Itoa(metadata.IssuingETHResponseMeta), strconv.Itoa(metadata.IssuingBSCResponseMeta)}
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
	if result == nil {
		currentState = State{}
		return nil
	}
	return database.DBUpdateProcessorState("shield", string(result))
}

func loadState() error {
	result, err := database.DBGetProcessorState("shield")
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
		switch metaDataType {
		case metadata.IssuingRequestMeta, metadata.IssuingBSCRequestMeta, metadata.IssuingETHRequestMeta:
			tokenIDStr := ""
			amount := uint64(0)
			switch metaDataType {
			case metadata.IssuingResponseMeta:
				meta := metadata.IssuingRequest{}
				err := json.Unmarshal([]byte(tx.Metadata), &meta)
				if err != nil {
					panic(err)
				}
				tokenIDStr = meta.TokenID.String()
				amount = meta.DepositedAmount
				bridge = "btc"
			case metadata.IssuingBSCRequestMeta, metadata.IssuingETHRequestMeta:
				meta := metadata.IssuingEVMRequest{}
				err := json.Unmarshal([]byte(tx.Metadata), &meta)
				if err != nil {
					panic(err)
				}
				tokenIDStr = meta.IncTokenID.String()
				if metaDataType == metadata.IssuingETHRequestMeta {
					bridge = "eth"
				} else {
					bridge = "bsc"
				}
			}
			shieldData := shared.NewShieldData(requestTx, "", tokenIDStr, bridge, "", isDecentralized, amount, tx.BlockHeight, tx.Locktime)
			requestData = append(respondData, *shieldData)
		case metadata.IssuingResponseMeta, metadata.IssuingETHResponseMeta, metadata.IssuingBSCResponseMeta:
			switch metaDataType {
			case metadata.IssuingResponseMeta:
				meta := metadata.IssuingResponse{}
				err := json.Unmarshal([]byte(tx.Metadata), &meta)
				if err != nil {
					panic(err)
				}
				requestTx = meta.RequestedTxID.String()
				bridge = "btc"
			case metadata.IssuingETHResponseMeta, metadata.IssuingBSCResponseMeta:
				meta := metadata.IssuingEVMResponse{}
				err := json.Unmarshal([]byte(tx.Metadata), &meta)
				if err != nil {
					panic(err)
				}
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
			shieldData := shared.NewShieldData(requestTx, tx.TxHash, tokenIDStr, bridge, "", isDecentralized, amount, tx.BlockHeight, 0)
			respondData = append(respondData, *shieldData)
		}

	}
	return requestData, respondData, nil
}
