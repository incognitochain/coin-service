package liquidity

import (
	"errors"
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
		time.Sleep(10 * time.Second)

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
	metas = append(metas, []string{strconv.Itoa(metadataCommon.PDEContributionMeta), strconv.Itoa(metadataCommon.PDEContributionResponseMeta), strconv.Itoa(metadataCommon.PDEWithdrawalRequestMeta), strconv.Itoa(metadataCommon.PDEWithdrawalResponseMeta), strconv.Itoa(metadataCommon.PDEFeeWithdrawalRequestMeta), strconv.Itoa(metadataCommon.PDEFeeWithdrawalResponseMeta)}...)
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
				ContributeAmount: []uint64{md.TokenAmount()},
				NFTID:            md.NftID(),
				PairHash:         md.PairHash(),
				RequestTime:      tx.Locktime,
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
				ReturnAmount: []uint64{amount},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawLiquidityRequest)
			data := shared.WithdrawContributionData{
				RequestTx:      tx.TxHash,
				NFTID:          md.NftID(),
				PoolID:         md.PoolPairID(),
				ShareAmount:    md.ShareAmount(),
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []uint64{},
				RequestTime:    tx.Locktime,
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
				WithdrawAmount: []uint64{amount},
				Status:         status,
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.WithdrawalLPFeeRequest)
			data := shared.WithdrawContributionFeeData{
				PoodID:         md.PoolPairID,
				NFTID:          md.NftID.String(),
				RequestTx:      tx.TxHash,
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []uint64{},
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
				WithdrawAmount: []uint64{amount},
				Status:         1,
			}
			withdrawFeeRespondDatas = append(withdrawFeeRespondDatas, data)
		case metadataCommon.Pdexv3StakingRequestMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingRequest)
			data := shared.PoolStakeHistoryData{
				RequestTx:   tx.TxHash,
				TokenID:     md.TokenID(),
				NFTID:       md.NftID(),
				Amount:      md.TokenAmount(),
				Status:      0,
				Requesttime: tx.Locktime,
				IsStaking:   true,
			}
			stakingRequestDatas = append(stakingRequestDatas, data)
		case metadataCommon.Pdexv3StakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.StakingResponse)
			status := 0
			if md.Status() == common.Pdexv3AcceptStakingStatus {
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
				NFTID:       md.NftID(),
				Amount:      md.UnstakingAmount(),
				Status:      0,
				Requesttime: tx.Locktime,
				IsStaking:   false,
			}
			stakingRequestDatas = append(stakingRequestDatas, data)
		case metadataCommon.Pdexv3UnstakingResponseMeta:
			md := txDetail.GetMetadata().(*metadataPdexv3.UnstakingResponse)
			status := 0
			if md.Status() == common.Pdexv3AcceptUnstakingStatus {
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
				NFTID:        md.NftID.String(),
				Requesttime:  tx.Locktime,
				RespondTxs:   []string{},
				Amount:       []uint64{},
				RewardTokens: []string{},
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
		case metadata.PDEContributionMeta:
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
				ContributeAmount: []uint64{md.ContributedAmount},
				RespondTxs:       []string{},
				ReturnTokens:     []string{},
				ReturnAmount:     []uint64{},
				RequestTime:      tx.Locktime,
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
				ReturnAmount: []uint64{amount},
			}
			contributeRespondDatas = append(contributeRespondDatas, data)
		case metadata.PDEWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEWithdrawalRequest)
			data := shared.WithdrawContributionData{
				RequestTx:      tx.TxHash,
				ShareAmount:    md.WithdrawalShareAmt,
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
				WithdrawTokens: []string{},
				WithdrawAmount: []uint64{},
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
				WithdrawAmount: []uint64{amount},
			}
			withdrawRespondDatas = append(withdrawRespondDatas, data)
		case metadata.PDEFeeWithdrawalRequestMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalRequest)
			data := shared.WithdrawContributionFeeData{
				RequestTx: tx.TxHash,
				// pdexv2 PRV only fee
				WithdrawTokens: []string{common.PRVCoinID.String()},
				WithdrawAmount: []uint64{md.WithdrawalFeeAmt},
				RequestTime:    tx.Locktime,
				Status:         0,
				RespondTxs:     []string{},
			}
			withdrawFeeRequestDatas = append(withdrawFeeRequestDatas, data)
		case metadata.PDEFeeWithdrawalResponseMeta:
			md := txDetail.GetMetadata().(*metadata.PDEFeeWithdrawalResponse)
			requestTx := md.RequestedTxID.String()
			data := shared.WithdrawContributionFeeData{
				RequestTx:  requestTx,
				RespondTxs: []string{tx.TxHash},
				Status:     1,
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
			if i.Status == common.Pdexv3RejectStakingStatus {
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
			if i.Status == common.Pdexv3RejectUnstakingStatus {
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
		list, err := database.DBGetRewardRecordByPoolID(poolid)
		if err != nil {
			return nil, err
		}
		var flist []shared.RewardRecord
		for _, v := range list {
			if v.BeaconHeight%config.Param().EpochParam.NumberOfBlockInEpoch == 0 {
				flist = append(flist, v)
			}
		}
		totalPercent := float64(0)
		for _, v := range flist {
			d := RewardInfo{}
			err := json.Unmarshal([]byte(v.Data), &d)
			if err != nil {
				return nil, err
			}
			if d.RewardReceiveInPRV > 0 && d.TotalAmountInPRV > 0 {
				totalPercent += (float64(d.RewardReceiveInPRV) / float64(d.TotalAmountInPRV) * 100)
			}
		}
		percent := totalPercent / float64(len(flist))
		if totalPercent != float64(0) {
			data.APY = uint64(percent * (365 * 86400 / config.Param().BlockTime.MinBeaconBlockInterval.Seconds() / float64(config.Param().EpochParam.NumberOfBlockInEpoch)))
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
