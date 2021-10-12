package chainsynker

import (
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	instruction "github.com/incognitochain/incognito-chain/instruction/pdexv3"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	log.Printf("start processing coin for block %v beacon\n", height)
	startTime := time.Now()
	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	blk := beaconBestState.BestBlock
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	var prevStateV2 *shared.PDEStateV2
	// this is a requirement check
	for shardID, blks := range blk.Body.ShardState {
		sort.Slice(blks, func(i, j int) bool { return blks[i].Height > blks[j].Height })
	retry:
		pass := true
		blockProcessedLock.RLock()
		if blockProcessed[int(shardID)] < blks[0].Height {
			pass = false
		}
		blockProcessedLock.RUnlock()
		if !pass {
			time.Sleep(1 * time.Second)
			goto retry
		}
	}
	if height != 1 {
		prevBeaconFeatureStateDB, err := Localnode.GetBlockchain().GetBestStateBeaconFeatureStateDBByHeight(height-1, Localnode.GetBlockchain().GetBeaconChainDatabase())
		if err != nil {
			log.Println(err)
		}

		pdeState, err := pdex.InitStateFromDB(prevBeaconFeatureStateDB, height-1, 2)
		if err != nil {
			log.Println(err)
		}
		poolPairs := make(map[string]*shared.PoolPairState)
		err = json.Unmarshal(pdeState.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}
		prevStateV2 = &shared.PDEStateV2{
			PoolPairs:         poolPairs,
			StakingPoolsState: pdeState.Reader().StakingPools(),
		}
	}
	// Process PDEstate
	stateV1, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight, 1)
	if err != nil {
		log.Println(err)
	}

	// var stateV1 *shared.PDEStateV1
	var stateV2 *shared.PDEStateV2

	// if pdeState.Version() == 1 {
	poolPairs := make(map[string]*rawdbv2.PDEPoolForPair)
	err = json.Unmarshal(stateV1.Reader().PoolPairs(), &poolPairs)
	if err != nil {
		panic(err)
	}
	waitingContributions := make(map[string]*rawdbv2.PDEContribution)
	err = json.Unmarshal(stateV1.Reader().WaitingContributions(), &waitingContributions)
	if err != nil {
		panic(err)
	}
	pdeStateJSON := jsonresult.CurrentPDEState{
		BeaconTimeStamp:         beaconBestState.BestBlock.Header.Timestamp,
		PDEPoolPairs:            poolPairs,
		PDEShares:               stateV1.Reader().Shares(),
		WaitingPDEContributions: waitingContributions,
		PDETradingFees:          stateV1.Reader().TradingFees(),
	}
	pdeStr, err := json.MarshalToString(pdeStateJSON)
	if err != nil {
		log.Println(err)
	}
	err = database.DBSavePDEState(pdeStr, 1)
	if err != nil {
		log.Println(err)
	}

	//process stateV2
	if beaconBestState.BeaconHeight >= config.Param().PDexParams.Pdexv3BreakPointHeight {
		pdeStateV2, err := pdex.InitStateFromDB(beaconFeatureStateDB, beaconBestState.BeaconHeight, 2)
		if err != nil {
			log.Println(err)
		}
		poolPairs := make(map[string]*shared.PoolPairState)
		err = json.Unmarshal(pdeStateV2.Reader().PoolPairs(), &poolPairs)
		if err != nil {
			panic(err)
		}
		poolPairsJSON := make(map[string]*pdex.PoolPairState)
		err = json.Unmarshal(pdeStateV2.Reader().PoolPairs(), &poolPairsJSON)
		if err != nil {
			panic(err)
		}

		stateV2 = &shared.PDEStateV2{
			PoolPairs:         poolPairs,
			StakingPoolsState: pdeStateV2.Reader().StakingPools(),
		}

		pdeStateJSON := jsonresult.Pdexv3State{
			BeaconTimeStamp: beaconBestState.BestBlock.Header.Timestamp,
			PoolPairs:       &poolPairsJSON,
			StakingPools:    &stateV2.StakingPoolsState,
		}
		pdeStr, err := json.MarshalToString(pdeStateJSON)
		if err != nil {
			log.Println(err)
		}

		pairDatas, poolDatas, sharesDatas, poolStakeDatas, poolStakersDatas, orderBook, poolDatasToBeDel, sharesDatasToBeDel, poolStakeDatasToBeDel, poolStakersDatasToBeDel, orderBookToBeDel, err := processPoolPairs(stateV2, prevStateV2, beaconBestState.BeaconHeight)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePDEState(pdeStr, 2)
		if err != nil {
			log.Println(err)
		}

		instructions, err := extractBeaconInstruction(beaconBestState.BestBlock.Body.Instructions)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolShareData(sharesDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolStakeData(poolStakeDatasToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolStakerData(poolStakersDatasToBeDel)
		if err != nil {
			panic(err)
		}
		err = database.DBDeleteOrderProgress(orderBookToBeDel)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveInstructionBeacon(instructions)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPairListData(pairDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolPairData(poolDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolShareData(sharesDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolStakeData(poolStakeDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdatePDEPoolStakerData(poolStakersDatas)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateOrderProgress(orderBook)
		if err != nil {
			panic(err)
		}

		err = database.DBDeletePDEPoolData(poolDatasToBeDel)
		if err != nil {
			panic(err)
		}
	}

	statePrefix := BeaconData
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	blockProcessedLock.Lock()
	blockProcessed[-1] = blk.Header.Height
	blockProcessedLock.Unlock()
	log.Printf("finish processing coin for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
}

func processPoolPairs(statev2 *shared.PDEStateV2, prevStatev2 *shared.PDEStateV2, beaconHeight uint64) ([]shared.PairData, []shared.PoolPairData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.PoolPairData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, error) {
	var pairList []shared.PairData
	pairListMap := make(map[string][]shared.PoolPairData)
	var poolPairs []shared.PoolPairData
	var poolShare []shared.PoolShareData
	var stakePools []shared.PoolStakeData
	var poolStaking []shared.PoolStakerData
	var orderStatus []shared.LimitOrderStatus

	var poolPairsToBeDelete []shared.PoolPairData
	var poolShareToBeDelete []shared.PoolShareData
	var stakePoolsToBeDelete []shared.PoolStakeData
	var poolStakingToBeDelete []shared.PoolStakerData
	var orderStatusToBeDelete []shared.LimitOrderStatus

	for poolID, state := range statev2.PoolPairs {
		poolData := shared.PoolPairData{
			Version:        2,
			PoolID:         poolID,
			PairID:         state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
			AMP:            state.State.Amplifier(),
			TokenID1:       state.State.Token0ID().String(),
			TokenID2:       state.State.Token1ID().String(),
			Token1Amount:   state.State.Token0RealAmount(),
			Token2Amount:   state.State.Token1RealAmount(),
			Virtual1Amount: state.State.Token0VirtualAmount().Uint64(),
			Virtual2Amount: state.State.Token1VirtualAmount().Uint64(),
			TotalShare:     state.State.ShareAmount(),
		}
		poolPairs = append(poolPairs, poolData)
		pairListMap[poolData.PairID] = append(pairListMap[poolData.PairID], poolData)
		for shareID, share := range state.Shares {
			tradingFee := make(map[string]uint64)
			for k, v := range share.TradingFees() {
				tradingFee[k.String()] = v
			}
			shareData := shared.PoolShareData{
				Version:    2,
				PoolID:     poolID,
				Amount:     share.Amount(),
				TradingFee: tradingFee,
				NFTID:      shareID,
			}
			poolShare = append(poolShare, shareData)
		}

		for _, order := range state.Orderbook.Orders {
			newOrder := shared.LimitOrderStatus{
				RequestTx:     order.Id(),
				Token1Balance: order.Token0Balance(),
				Token2Balance: order.Token1Balance(),
				Direction:     order.TradeDirection(),
			}
			orderStatus = append(orderStatus, newOrder)
		}
	}

	for pairID, pools := range pairListMap {
		data := shared.PairData{
			PairID:    pairID,
			TokenID1:  pools[0].TokenID1,
			TokenID2:  pools[0].TokenID2,
			PoolCount: len(pools),
		}

		for _, v := range pools {
			data.Token1Amount += v.Token1Amount
			data.Token2Amount += v.Token2Amount
		}

		pairList = append(pairList, data)
	}

	for tokenID, stakeData := range statev2.StakingPoolsState {
		poolData := shared.PoolStakeData{
			Amount:  stakeData.Liquidity(),
			TokenID: tokenID,
		}
		stakePools = append(stakePools, poolData)
		for nftID, staker := range stakeData.Stakers() {
			rewardMap := make(map[string]uint64)
			for k, v := range staker.Rewards() {
				rewardMap[k.String()] = v
			}
			stake := shared.PoolStakerData{
				TokenID: tokenID,
				NFTID:   nftID,
				Amount:  stakeData.Liquidity(),
				Reward:  rewardMap,
			}
			poolStaking = append(poolStaking, stake)
		}
	}

	//comparing with old state
	if prevStatev2 != nil {
		for poolID, state := range prevStatev2.PoolPairs {
			willDelete := false
			if _, ok := statev2.PoolPairs[poolID]; !ok {
				willDelete = true
			}
			if willDelete {
				poolData := shared.PoolPairData{
					Version: 2,
					PoolID:  poolID,
					PairID:  state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
				}
				poolPairsToBeDelete = append(poolPairsToBeDelete, poolData)
				for shareID, _ := range state.Shares {
					shareData := shared.PoolShareData{
						PoolID:  poolID,
						NFTID:   shareID,
						Version: 2,
					}
					poolShareToBeDelete = append(poolShareToBeDelete, shareData)
				}
				for _, order := range state.Orderbook.Orders {
					newOrder := shared.LimitOrderStatus{
						RequestTx: order.Id(),
					}
					orderStatusToBeDelete = append(orderStatusToBeDelete, newOrder)
				}
			} else {
				newState := statev2.PoolPairs[poolID]
				for shareID, _ := range state.Shares {
					willDelete := false
					if _, ok := newState.Shares[shareID]; !ok {
						willDelete = true
					}
					if willDelete {
						shareData := shared.PoolShareData{
							PoolID:  poolID,
							NFTID:   shareID,
							Version: 2,
						}
						poolShareToBeDelete = append(poolShareToBeDelete, shareData)
					}
				}
				for _, order := range state.Orderbook.Orders {
					willDelete := true
					for _, v := range newState.Orderbook.Orders {
						if v.Id() == order.Id() {
							willDelete = false
						}
					}
					if willDelete {
						newOrder := shared.LimitOrderStatus{
							RequestTx: order.Id(),
						}
						orderStatusToBeDelete = append(orderStatusToBeDelete, newOrder)
					}
				}
			}

		}

		for tokenID, stakeData := range prevStatev2.StakingPoolsState {
			willDelete := false
			if _, ok := statev2.StakingPoolsState[tokenID]; !ok {
				willDelete = true
			}
			if willDelete {
				poolData := shared.PoolStakeData{
					TokenID: tokenID,
				}
				stakePoolsToBeDelete = append(stakePoolsToBeDelete, poolData)
				for nftID, _ := range stakeData.Stakers() {
					stake := shared.PoolStakerData{
						TokenID: tokenID,
						NFTID:   nftID,
					}
					poolStakingToBeDelete = append(poolStakingToBeDelete, stake)
				}
			} else {
				newStaker := statev2.StakingPoolsState[tokenID].Stakers()
				for nftID, _ := range stakeData.Stakers() {
					willDelete := false
					if _, ok := newStaker[nftID]; !ok {
						willDelete = true
					}
					if willDelete {
						stake := shared.PoolStakerData{
							TokenID: tokenID,
							NFTID:   nftID,
						}
						poolStakingToBeDelete = append(poolStakingToBeDelete, stake)
					}
				}
			}
		}
	}

	return pairList, poolPairs, poolShare, stakePools, poolStaking, orderStatus, poolPairsToBeDelete, poolShareToBeDelete, stakePoolsToBeDelete, poolStakingToBeDelete, orderStatusToBeDelete, nil
}

func extractBeaconInstruction(insts [][]string) ([]shared.InstructionBeaconData, error) {
	var result []shared.InstructionBeaconData
	for _, inst := range insts {
		data := shared.InstructionBeaconData{}

		metadataType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue // Not error, just not PDE instructions
		}
		data.Metatype = inst[0]
		switch metadataType {
		case metadata.PDEWithdrawalRequestMeta:
			contentBytes, err := base64.StdEncoding.DecodeString(inst[3])
			if err != nil {
				panic(err)
			}
			var withdrawalRequestAction metadata.PDEWithdrawalRequestAction
			err = json.Unmarshal(contentBytes, &withdrawalRequestAction)
			if err != nil {
				panic(err)
			}
			data.Content = inst[3]
			data.Status = inst[2]
			data.TxRequest = withdrawalRequestAction.TxReqID.String()
		case metadata.PDEFeeWithdrawalRequestMeta:
			contentStr := inst[3]
			contentBytes, err := base64.StdEncoding.DecodeString(contentStr)
			if err != nil {
				panic(err)
			}
			var feeWithdrawalRequestAction metadata.PDEFeeWithdrawalRequestAction
			err = json.Unmarshal(contentBytes, &feeWithdrawalRequestAction)
			if err != nil {
				panic(err)
			}
			data.Content = inst[3]
			data.Status = inst[2]
			data.TxRequest = feeWithdrawalRequestAction.TxReqID.String()

		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.PDEWithdrawalRejectedChainStatus:
				rejectWithdrawLiquidity := instruction.NewRejectWithdrawLiquidity()
				err := rejectWithdrawLiquidity.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectWithdrawLiquidity.TxReqID().String()
			case common.PDEWithdrawalAcceptedChainStatus:
				acceptWithdrawLiquidity := instruction.NewAcceptWithdrawLiquidity()
				err := acceptWithdrawLiquidity.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptWithdrawLiquidity.TxReqID().String()
			}
		// case metadataCommon.Pdexv3TradeRequestMeta:

		case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
			data.Status = inst[2]
			data.Content = inst[3]
			var actionData metadataPdexv3.WithdrawalLPFeeContent
			err := json.Unmarshal([]byte(inst[3]), &actionData)
			if err != nil {
				panic(err)
			}
			data.TxRequest = actionData.TxReqID.String()
		// case metadataCommon.Pdexv3WithdrawProtocolFeeRequestMeta:
		// 	data.Status = inst[2]
		// 	data.Content = inst[3]
		// 	var actionData metadataPdexv3.WithdrawalProtocolFeeContent
		// 	err := json.Unmarshal([]byte(inst[3]), &actionData)
		// 	if err != nil {
		// 		panic(err)
		// 	}
		// 	data.TxRequest = actionData.TxReqID.String()
		// case metadataCommon.Pdexv3AddOrderRequestMeta:

		case metadataCommon.Pdexv3WithdrawOrderRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case strconv.Itoa(metadataPdexv3.WithdrawOrderAcceptedStatus):
				currentOrder := &instruction.Action{Content: &metadataPdexv3.AcceptedWithdrawOrder{}}
				err := currentOrder.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = currentOrder.RequestTxID().String()
			case strconv.Itoa(metadataPdexv3.WithdrawOrderRejectedStatus):
				currentOrder := &instruction.Action{Content: &metadataPdexv3.RejectedWithdrawOrder{}}
				err := currentOrder.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = currentOrder.RequestTxID().String()
			}
		// case metadataCommon.Pdexv3DistributeStakingRewardMeta:

		// case metadataCommon.Pdexv3StakingRequestMeta:

		case metadataCommon.Pdexv3UnstakingRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.Pdexv3AcceptUnstakingStatus:
				acceptInst := instruction.NewAcceptUnstaking()
				err := acceptInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptInst.TxReqID().String()
			case common.Pdexv3RejectUnstakingStatus:
				rejectInst := instruction.NewRejectUnstaking()
				err := rejectInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectInst.TxReqID().String()
			}
		case metadataCommon.Pdexv3WithdrawStakingRewardRequestMeta:
			var actionData metadataPdexv3.WithdrawalStakingRewardContent
			err := json.Unmarshal([]byte(inst[3]), &actionData)
			if err != nil {
				panic(err)
			}
			data.Status = inst[2]
			data.Content = inst[3]
			data.TxRequest = actionData.TxReqID.String()
		default:
			continue
		}

		result = append(result, data)
	}
	return result, nil
}
