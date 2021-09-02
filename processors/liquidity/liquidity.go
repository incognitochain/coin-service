package liquidity

import (
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/transaction"
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

		contribRQData, contribRPData, withdrawRQDatas, withdrawRPDatas, withdrawFeeRQDatas, withdrawFeeRPDatas, err := processAddLiquidity(txList)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePDEContribute(contribRQData)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePDEWithdraw(withdrawRQDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePDEWithdrawFee(withdrawFeeRQDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEContribute(contribRPData)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEWithdraw(withdrawRPDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEWithdrawFee(withdrawFeeRPDatas)
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

func processAddLiquidity(txList []shared.TxData) ([]shared.ContributionData, []shared.ContributionData, []shared.WithdrawContributionData, []shared.WithdrawContributionData, []shared.WithdrawContributionFeeData, []shared.WithdrawContributionFeeData, error) {

	var contributeRequestDatas []shared.ContributionData
	var contributeRespondDatas []shared.ContributionData

	var withdrawRequestDatas []shared.WithdrawContributionData
	var withdrawRespondDatas []shared.WithdrawContributionData

	var withdrawFeeRequestDatas []shared.WithdrawContributionFeeData
	var withdrawFeeRespondDatas []shared.WithdrawContributionFeeData

	for _, tx := range txList {
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
			data := shared.ContributionData{
				RequestTx:        tx.TxHash,
				PoolID:           md.PoolPairID(),
				ContributeToken:  []string{md.TokenID()},
				ContributeAmount: []uint64{md.TokenAmount()},
				NFTID:            md.NftID(),
				PairHash:         md.PairHash(),
			}
			contributeRequestDatas = append(contributeRequestDatas, data)

		case metadataCommon.Pdexv3AddLiquidityResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.AddLiquidityResponse)
			requestTx := md.TxReqID()
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
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				amount = outs[0].GetValue()
			}
			data := shared.ContributionData{
				RequestTx:    requestTx,
				RespondTxs:   []string{tx.TxHash},
				ReturnToken:  []string{tokenIDStr},
				ReturnAmount: []uint64{amount},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityRequest)
			data := shared.WithdrawContributionData{
				RequestTx:  tx.TxHash,
				NFTID:      md.NftID(),
				RespondTxs: []string{tx.TxHash},
				PoolID:     md.PoolPairID(),
			}
			withdrawRequestDatas = append(withdrawRequestDatas, data)
		case metadataCommon.Pdexv3WithdrawLiquidityResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityResponse)
			data := shared.WithdrawContributionData{
				RequestTx:  md.TxReqID(),
				RespondTxs: []string{tx.TxHash},
				// Amount: md.,
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadata.PDEContributionMeta:
			md := txDetail.GetMetadata().(*metadata.PDEContribution)
			data := shared.ContributionData{
				RequestTx:        tx.TxHash,
				PairID:           md.PDEContributionPairID,
				ContributeToken:  []string{md.TokenIDStr},
				ContributeAmount: []uint64{md.ContributedAmount},
			}
			contributeRequestDatas = append(contributeRequestDatas, data)
		case metadata.PDEWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEWithdrawalRequest)
			data := shared.WithdrawContributionData{
				RequestTx: tx.TxHash,
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
			_ = md
		case metadata.PDEFeeWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalRequest)
			data := shared.WithdrawContributionFeeData{
				RequestTx: tx.TxHash,
			}
			withdrawFeeRequestDatas = append(withdrawFeeRequestDatas, data)
			_ = md
		case metadata.PDEContributionResponseMeta:
			md := txDetail.GetMetadata().(*metadata.PDEContributionResponse)
			requestTx := md.RequestedTxID.String()
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
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				amount = outs[0].GetValue()
			}
			data := shared.ContributionData{
				RequestTx:    requestTx,
				RespondTxs:   []string{tx.TxHash},
				ReturnToken:  []string{tokenIDStr},
				ReturnAmount: []uint64{amount},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadata.PDEWithdrawalResponseMeta:
			md := txDetail.GetMetadata().(*metadata.PDEWithdrawalResponse)
			requestTx := md.RequestedTxID.String()
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
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				amount = outs[0].GetValue()
			}
			data := shared.WithdrawContributionData{
				RequestTx:      requestTx,
				RespondTxs:     []string{tx.TxHash},
				Status:         "success",
				WithdrawToken:  []string{tokenIDStr},
				WithdrawAmount: []uint64{amount},
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadata.PDEFeeWithdrawalResponseMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalResponse)
			requestTx := md.RequestedTxID.String()
			amount := uint64(0)
			outs := txDetail.GetProof().GetOutputCoins()
			amount = outs[0].GetValue()

			data := shared.WithdrawContributionFeeData{
				RequestTx:  requestTx,
				RespondTxs: []string{tx.TxHash},
				Status:     "success",
				// pdexv2 PRV only fee
				ReturnTokens: []string{common.PRVCoinID.String()},
				ReturnAmount: []uint64{amount},
			}
			withdrawFeeRespondDatas = append(withdrawFeeRespondDatas, data)
			// case pdexV3 Staking
		}
	}

	return contributeRequestDatas, contributeRespondDatas, withdrawRequestDatas, withdrawRespondDatas, withdrawFeeRequestDatas, withdrawFeeRespondDatas, nil
}
