package liquidity

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

		txList, err := getTxToProcess(currentState.LastProcessedObjectID, 1000)
		if err != nil {
			log.Println("getTxToProcess", err)
			continue
		}
		log.Println("start processing LQ with", len(txList), "txs")
		pdexState, beaconHeight, err := getPdexToProcess(currentState.LastProcessedPdexV3Height)
		if err != nil {
			log.Println("getPdexToProcess 2", err)
			continue
		}

		var apyData []shared.RewardAPYTracking

		if pdexState != nil {
			apyData, err = processPoolRewardAPY(pdexState, beaconHeight)
			if err != nil {
				log.Println("getPdexToProcess", err)
				continue
			}
			currentState.LastProcessedPdexV3Height = beaconHeight
		}
		contribRQData, contribRPData, withdrawRQDatas, withdrawRPDatas, withdrawFeeRQDatas, withdrawFeeRPDatas, stakeRQDatas, stakeRPDatas, stakeRewardRQDatas, stakeRewardRPDatas, err := processLiquidity(txList)
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

		err = database.DBSavePDEStakeHistory(stakeRQDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePDEStakeRewardHistory(stakeRewardRQDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolPairRewardAPY(apyData)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEContributeRespond(contribRPData)
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

		err = database.DBUpdatePDEStakingHistory(stakeRPDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEStakeRewardHistory(stakeRewardRPDatas)
		if err != nil {
			panic(err)
		}

		log.Println("finish processing LQ with", len(txList), "txs")
		if len(txList) != 0 {
			currentState.LastProcessedObjectID = txList[len(txList)-1].ID.Hex()
			err = updateState()
			if err != nil {
				panic(err)
			}
		}
		err = updateLiquidityStatus()
		if err != nil {
			panic(err)
		}
	}
}

func getTxToProcess(lastID string, limit int64) ([]shared.TxData, error) {
	var result []shared.TxData
	metas := []string{strconv.Itoa(metadataCommon.Pdexv3AddLiquidityRequestMeta), strconv.Itoa(metadataCommon.Pdexv3AddLiquidityResponseMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLiquidityRequestMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLiquidityResponseMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLPFeeRequestMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawLPFeeResponseMeta), strconv.Itoa(metadataCommon.Pdexv3StakingRequestMeta), strconv.Itoa(metadataCommon.Pdexv3StakingResponseMeta), strconv.Itoa(metadataCommon.Pdexv3UnstakingRequestMeta), strconv.Itoa(metadataCommon.Pdexv3UnstakingResponseMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawStakingRewardRequestMeta), strconv.Itoa(metadataCommon.Pdexv3WithdrawStakingRewardResponseMeta)}
	metas = append(metas, []string{strconv.Itoa(metadataCommon.PDEContributionMeta), strconv.Itoa(metadataCommon.PDEContributionResponseMeta), strconv.Itoa(metadataCommon.PDEWithdrawalRequestMeta), strconv.Itoa(metadataCommon.PDEWithdrawalResponseMeta), strconv.Itoa(metadataCommon.PDEFeeWithdrawalRequestMeta), strconv.Itoa(metadataCommon.PDEFeeWithdrawalResponseMeta), strconv.Itoa(metadataCommon.PDEPRVRequiredContributionRequestMeta)}...)
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
	err := mgm.Coll(&shared.TxData{}).SimpleFindWithCtx(context.Background(), &result, filter, &options.FindOptions{
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
	if result == nil {
		currentState = State{}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}

func processLiquidity(txList []shared.TxData) ([]shared.ContributionData, []shared.ContributionData, []shared.WithdrawContributionData, []shared.WithdrawContributionData, []shared.WithdrawContributionFeeData, []shared.WithdrawContributionFeeData, []shared.PoolStakeHistoryData, []shared.PoolStakeHistoryData, []shared.PoolStakeRewardHistoryData, []shared.PoolStakeRewardHistoryData, error) {

	var contributeRequestDatas []shared.ContributionData
	var contributeRespondDatas []shared.ContributionData

	var withdrawRequestDatas []shared.WithdrawContributionData
	var withdrawRespondDatas []shared.WithdrawContributionData

	var withdrawFeeRequestDatas []shared.WithdrawContributionFeeData
	var withdrawFeeRespondDatas []shared.WithdrawContributionFeeData

	var stakingRequestDatas []shared.PoolStakeHistoryData
	var stakingRespondDatas []shared.PoolStakeHistoryData

	var stakingRewardRequestDatas []shared.PoolStakeRewardHistoryData
	var stakingRewardRespondDatas []shared.PoolStakeRewardHistoryData

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

		//---------------------------------------------------
		//PDexV3
		case metadataCommon.Pdexv3AddLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.AddLiquidityRequest)
			data := shared.ContributionData{
				RequestTxs:       []string{tx.TxHash},
				PoolID:           md.PoolPairID(),
				ContributeTokens: []string{md.TokenID()},
				ContributeAmount: []string{fmt.Sprintf("%v", md.TokenAmount())},
				PairHash:         md.PairHash(),
				RequestTime:      tx.Locktime,
				Version:          3,
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
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
				RequestTxs:   []string{requestTx},
				RespondTxs:   []string{tx.TxHash},
				ReturnTokens: []string{tokenIDStr},
				ReturnAmount: []string{fmt.Sprintf("%v", amount)},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityRequest)
			data := shared.WithdrawContributionData{
				RequestTx:      tx.TxHash,
				PoolID:         md.PoolPairID(),
				ShareAmount:    fmt.Sprintf("%v", md.ShareAmount()),
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []string{},
				RequestTime:    tx.Locktime,
				Version:        3,
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
			}
			withdrawRequestDatas = append(withdrawRequestDatas, data)
		case metadataCommon.Pdexv3WithdrawLiquidityResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityResponse)
			status := 0
			if md.Status() == common.PDEWithdrawalAcceptedChainStatus {
				status = 1
			} else {
				status = 2
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
			} else {
				outs := txDetail.GetProof().GetOutputCoins()
				amount = outs[0].GetValue()
			}
			data := shared.WithdrawContributionData{
				RequestTx:      md.TxReqID(),
				RespondTxs:     []string{tx.TxHash},
				WithdrawTokens: []string{tokenIDStr},
				WithdrawAmount: []string{fmt.Sprintf("%v", amount)},
				Status:         status,
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawalLPFeeRequest)
			data := shared.WithdrawContributionFeeData{
				PoodID:         md.PoolPairID,
				RequestTx:      tx.TxHash,
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []string{},
				Version:        3,
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
			}
			withdrawFeeRequestDatas = append(withdrawFeeRequestDatas, data)
		case metadataCommon.Pdexv3WithdrawLPFeeResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawalLPFeeResponse)
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
			data := shared.WithdrawContributionFeeData{
				RequestTx:      md.ReqTxID.String(),
				RespondTxs:     []string{tx.TxHash},
				WithdrawTokens: []string{tokenIDStr},
				WithdrawAmount: []string{fmt.Sprintf("%v", amount)},
				Status:         1,
			}
			withdrawFeeRespondDatas = append(withdrawFeeRespondDatas, data)
		case metadataCommon.Pdexv3StakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingRequest)
			data := shared.PoolStakeHistoryData{
				RequestTx:   tx.TxHash,
				TokenID:     md.TokenID(),
				Amount:      md.TokenAmount(),
				Status:      0,
				Requesttime: tx.Locktime,
				IsStaking:   true,
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
			}
			stakingRequestDatas = append(stakingRequestDatas, data)
		case metadataCommon.Pdexv3StakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingResponse)
			status := 0
			if md.Status() == common.Pdexv3AcceptStringStatus {
				status = 1
			} else {
				status = 2
			}
			data := shared.PoolStakeHistoryData{
				RequestTx: md.TxReqID(),
				RespondTx: tx.TxHash,
				Status:    status,
			}
			stakingRespondDatas = append(stakingRespondDatas, data)
		case metadataCommon.Pdexv3UnstakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.UnstakingRequest)
			data := shared.PoolStakeHistoryData{
				RequestTx:   tx.TxHash,
				TokenID:     md.StakingPoolID(),
				Amount:      md.UnstakingAmount(),
				Status:      0,
				Requesttime: tx.Locktime,
				IsStaking:   false,
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
			}
			stakingRequestDatas = append(stakingRequestDatas, data)
		case metadataCommon.Pdexv3UnstakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.UnstakingResponse)
			status := 0
			if md.Status() == common.Pdexv3AcceptStringStatus {
				status = 1
			} else {
				status = 2
			}
			data := shared.PoolStakeHistoryData{
				RequestTx: md.TxReqID(),
				RespondTx: tx.TxHash,
				Status:    status,
			}
			stakingRespondDatas = append(stakingRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawStakingRewardRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawalStakingRewardRequest)
			data := shared.PoolStakeRewardHistoryData{
				RequestTx:    tx.TxHash,
				Status:       0,
				TokenID:      md.StakingPoolID,
				Requesttime:  tx.Locktime,
				RespondTxs:   []string{},
				Amount:       []uint64{},
				RewardTokens: []string{},
			}
			if md.UseNft() {
				data.NFTID = md.NftID.String()
			} else {
				data.NFTID = md.AccessID.String()
			}
			stakingRewardRequestDatas = append(stakingRewardRequestDatas, data)
		case metadataCommon.Pdexv3WithdrawStakingRewardResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawalStakingRewardResponse)
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
			data := shared.PoolStakeRewardHistoryData{
				RequestTx:    md.ReqTxID.String(),
				RespondTxs:   []string{tx.TxHash},
				Status:       1,
				Amount:       []uint64{amount},
				RewardTokens: []string{tokenIDStr},
			}
			stakingRewardRespondDatas = append(stakingRewardRespondDatas, data)
		//---------------------------------------------------
		//PDexV2
		case metadata.PDEContributionMeta, metadata.PDEPRVRequiredContributionRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEContribution)
			wl, err := wallet.Base58CheckDeserialize(md.ContributorAddressStr)
			if err != nil {
				panic(err)
			}
			pubkey := base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
			data := shared.ContributionData{
				Contributor:      pubkey,
				RequestTxs:       []string{tx.TxHash},
				PairID:           md.PDEContributionPairID,
				ContributeTokens: []string{md.TokenIDStr},
				ContributeAmount: []string{fmt.Sprintf("%v", md.ContributedAmount)},
				RespondTxs:       []string{},
				ReturnTokens:     []string{},
				ReturnAmount:     []string{},
				RequestTime:      tx.Locktime,
				Version:          2,
			}
			contributeRequestDatas = append(contributeRequestDatas, data)
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
				RequestTxs:   []string{requestTx},
				RespondTxs:   []string{tx.TxHash},
				ReturnTokens: []string{tokenIDStr},
				ReturnAmount: []string{fmt.Sprintf("%v", amount)},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadata.PDEWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEWithdrawalRequest)
			data := shared.WithdrawContributionData{
				RequestTx:      tx.TxHash,
				ShareAmount:    fmt.Sprintf("%v", md.WithdrawalShareAmt),
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []string{},
				Version:        2,
			}
			withdrawRequestDatas = append(withdrawRequestDatas, data)
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
				Status:         1,
				WithdrawTokens: []string{tokenIDStr},
				WithdrawAmount: []string{fmt.Sprintf("%v", amount)},
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadata.PDEFeeWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalRequest)
			data := shared.WithdrawContributionFeeData{
				RequestTx: tx.TxHash,
				// pdexv2 PRV only fee
				WithdrawTokens: []string{common.PRVCoinID.String()},
				WithdrawAmount: []string{fmt.Sprintf("%v", md.WithdrawalFeeAmt)},
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
				Version:        2,
			}
			withdrawFeeRequestDatas = append(withdrawFeeRequestDatas, data)
		case metadata.PDEFeeWithdrawalResponseMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalResponse)
			requestTx := md.RequestedTxID.String()
			data := shared.WithdrawContributionFeeData{
				RequestTx:      requestTx,
				RespondTxs:     []string{tx.TxHash},
				Status:         1,
				WithdrawTokens: []string{},
				WithdrawAmount: []string{},
			}
			withdrawFeeRespondDatas = append(withdrawFeeRespondDatas, data)
		}
	}

	return contributeRequestDatas, contributeRespondDatas, withdrawRequestDatas, withdrawRespondDatas, withdrawFeeRequestDatas, withdrawFeeRespondDatas, stakingRequestDatas, stakingRespondDatas, stakingRewardRequestDatas, stakingRewardRespondDatas, nil
}

func updateLiquidityStatus() error {
	limit := int64(10000)
	offset := int64(0)

	for {
		list, err := database.DBGetPendingLiquidityWithdraw(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.WithdrawContributionData{}
		for _, v := range list {
			data := shared.WithdrawContributionData{
				RequestTx: v.RequestTx,
			}
			i, err := database.DBGetBeaconInstructionByTx(v.RequestTx)
			if i == nil && err == nil {
				continue
			}
			if err != nil {
				panic(err)
			}
			if i.Status == common.PDEWithdrawalRejectedChainStatus {
				data.Status = 2
				listToUpdate = append(listToUpdate, data)
			}
		}
		err = database.DBUpdatePDELiquidityWithdrawStatus(listToUpdate)
		if err != nil {
			return err
		}
	}

	offset = 0
	for {
		list, err := database.DBGetPendingLiquidityWithdrawFee(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.WithdrawContributionFeeData{}
		for _, v := range list {
			data := shared.WithdrawContributionFeeData{
				RequestTx: v.RequestTx}
			i, err := database.DBGetBeaconInstructionByTx(v.RequestTx)
			if i == nil && err == nil {
				continue
			}
			if err != nil {
				panic(err)
			}
			if i.Status == metadataPdexv3.RequestRejectedChainStatus {
				data.Status = 2
				listToUpdate = append(listToUpdate, data)
			}
		}
		err = database.DBUpdatePDELiquidityWithdrawFeeStatus(listToUpdate)
		if err != nil {
			return err
		}
	}

	offset = 0
	for {
		list, err := database.DBGetPendingRequestStakingPool(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.PoolStakeHistoryData{}
		for _, v := range list {
			data := shared.PoolStakeHistoryData{
				RequestTx: v.RequestTx}
			i, err := database.DBGetBeaconInstructionByTx(v.RequestTx)
			if i == nil && err == nil {
				continue
			}
			if err != nil {
				panic(err)
			}
			if i.Status == common.Pdexv3RejectStringStatus {
				data.Status = 2
				listToUpdate = append(listToUpdate, data)
			} else {
				data.Status = 1
				listToUpdate = append(listToUpdate, data)
			}
		}
		err = database.DBUpdateRequestStakingPoolStatus(listToUpdate)
		if err != nil {
			return err
		}
	}

	offset = 0
	for {
		list, err := database.DBGetPendingUnstakingPool(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.PoolStakeHistoryData{}
		for _, v := range list {
			data := shared.PoolStakeHistoryData{
				RequestTx: v.RequestTx}
			i, err := database.DBGetBeaconInstructionByTx(v.RequestTx)
			if i == nil && err == nil {
				continue
			}
			if err != nil {
				panic(err)
			}
			if i.Status == common.Pdexv3RejectStringStatus {
				data.Status = 2
				listToUpdate = append(listToUpdate, data)
			}
		}
		err = database.DBUpdatePDEUnstakingPoolStatus(listToUpdate)
		if err != nil {
			return err
		}
	}

	offset = 0
	for {
		list, err := database.DBGetPendingWithdrawRewardStakingPool(limit, offset)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			break
		}
		offset += int64(len(list))
		listToUpdate := []shared.PoolStakeRewardHistoryData{}
		for _, v := range list {
			data := shared.PoolStakeRewardHistoryData{
				RequestTx: v.RequestTx}
			i, err := database.DBGetBeaconInstructionByTx(v.RequestTx)
			if i == nil && err == nil {
				continue
			}
			if err != nil {
				panic(err)
			}
			if i.Status == metadataPdexv3.RequestRejectedChainStatus {
				data.Status = 2
				listToUpdate = append(listToUpdate, data)
			}
		}
		err = database.DBUpdatePDEWithdrawRewardStakingStatus(listToUpdate)
		if err != nil {
			return err
		}
	}

	return nil
}

func getPdexToProcess(height uint64) (*jsonresult.Pdexv3State, uint64, error) {
	data, err := database.DBGetPDEStateWithHeight(2, height+1)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	pdeState := jsonresult.Pdexv3State{}
	err = json.UnmarshalFromString(data.State, &pdeState)
	if err != nil {
		return nil, 0, err
	}
	return &pdeState, data.Height, nil
}

func processPoolRewardAPY(pdex *jsonresult.Pdexv3State, height uint64) ([]shared.RewardAPYTracking, error) {
	var result []shared.RewardAPYTracking
	for poolid, _ := range *pdex.PoolPairs {
		data := shared.RewardAPYTracking{
			DataID:       poolid,
			BeaconHeight: height,
		}
		blocks := int64((86400 / config.Param().BlockTime.MinBeaconBlockInterval.Seconds()) * 7)
		if blocks > 50000 {
			blocks = 50000
		}
		list, err := database.DBGetRewardRecordByPoolID(poolid, blocks)
		if err != nil {
			return nil, err
		}
		totalAmount := uint64(0)
		totalReceive := uint64(0)
		totalPercent := float64(0)
		heightBc := uint64(0)
		for _, v := range list {
			d := RewardInfo{}
			err := json.Unmarshal([]byte(v.Data), &d)
			if err != nil {
				return nil, err
			}
			if d.RewardReceiveInPRV > 0 && d.TotalAmountInPRV > 0 {
				totalPercent += (float64(d.RewardReceiveInPRV) / float64(d.TotalAmountInPRV) * 100)
			}
			totalReceive += d.RewardReceiveInPRV
			if v.BeaconHeight > heightBc {
				totalAmount = d.TotalAmountInPRV
				heightBc = v.BeaconHeight
			}
		}
		data.TotalAmount = totalAmount
		data.TotalReceive = totalReceive
		apy2 := ((float64(totalReceive) / float64(totalAmount)) * 100 / float64(len(list))) * ((365 * 86400) / config.Param().BlockTime.MinBeaconBlockInterval.Seconds())
		data.APY2 = apy2
		fmt.Println("poolid", poolid, apy2, totalReceive, totalAmount)
		percent := totalPercent / float64(len(list))
		if totalPercent != float64(0) {
			p := (percent * ((365 * 86400) / config.Param().BlockTime.MinBeaconBlockInterval.Seconds()))
			// data.APY = uint64(math.Pow(float64(1+p/12), 12) - 1)
			data.APY = p
		}
		result = append(result, data)
	}
	for poolid, _ := range *pdex.StakingPools {
		data := shared.RewardAPYTracking{
			DataID:       poolid,
			BeaconHeight: height,
		}
		blocks := int64((86400 / config.Param().BlockTime.MinBeaconBlockInterval.Seconds()) * 7)
		if blocks > 50000 {
			blocks = 50000
		}
		list, err := database.DBGetRewardRecordByPoolID(poolid, blocks)
		if err != nil {
			return nil, err
		}
		totalPercent := float64(0)
		// totalLen := 0
		totalAmount := uint64(0)
		totalReceive := uint64(0)
		heightBc := uint64(0)
		for _, v := range list {
			d := RewardInfo{}
			err := json.Unmarshal([]byte(v.Data), &d)
			if err != nil {
				return nil, err
			}
			if d.RewardReceiveInPRV > 0 && d.TotalAmountInPRV > 0 {
				totalPercent += (float64(d.RewardReceiveInPRV) / float64(d.TotalAmountInPRV) * 100)
			}
			totalReceive += d.RewardReceiveInPRV
			if v.BeaconHeight > heightBc {
				totalAmount = d.TotalAmountInPRV
				heightBc = v.BeaconHeight
			}
		}
		data.TotalAmount = totalAmount
		data.TotalReceive = totalReceive
		apy2 := ((float64(totalReceive) / float64(totalAmount)) * 100 / float64(len(list))) * ((365 * 86400) / config.Param().BlockTime.MinBeaconBlockInterval.Seconds())
		data.APY2 = apy2
		percent := totalPercent / float64(len(list))
		if totalPercent != float64(0) {
			p := (percent * ((365 * 86400) / config.Param().BlockTime.MinBeaconBlockInterval.Seconds()))
			// data.APY = uint64(math.Pow(float64(1+p/12), 12) - 1)
			data.APY = p
		}
		result = append(result, data)
	}
	h1 := uint64((86400 / config.Param().BlockTime.MinBeaconBlockInterval.Seconds()) * 7)
	if height > h1 {
		err := database.DBDeleteRewardRecord(height - h1)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
