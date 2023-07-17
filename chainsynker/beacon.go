package chainsynker

import (
	"encoding/base64"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
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

type IncPdexState struct {
	StakingPools map[string]*pdex.StakingPoolState
	PoolPairs    map[string]*pdex.PoolPairState
	Param        *statedb.Pdexv3Params
}

var pdexV3State pdex.State

// var bridgeManager *bridgeagg.Manager

func processBeacon(bc *blockchain.BlockChain, h common.Hash, height uint64, chainID int) {
	willPauseOperation(-1)
	log.Printf("start processing coin for block %v beacon\n", height)
	blk, _, err := Localnode.GetBlockchain().GetBeaconBlockByHash(h)
	if err != nil {
		panic(err)
	}
	// this is a requirement check
	for shardID, blks := range blk.Body.ShardState {
		sort.Slice(blks, func(i, j int) bool { return blks[i].Height > blks[j].Height })
	retry:
		pass := true
		currentState.blockProcessedLock.RLock()
		if currentState.BlockProcessed[int(shardID)] < blks[0].Height {
			pass = false
		}
		currentState.blockProcessedLock.RUnlock()
		if !pass {
			time.Sleep(500 * time.Millisecond)
			goto retry
		}
	}
	startTime := time.Now()
	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false, false, true)
	beaconFeatureStateDB := beaconBestState.GetBeaconFeatureStateDB()
	// if bridgeManager == nil {
	// 	bridgeManager, err = bridgeagg.InitStateFromDB(beaconFeatureStateDB)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// } else {
	// 	bridgeManager.ClearCache()
	// 	err = bridgeManager.Process(blk.Body.Instructions, beaconFeatureStateDB)
	// 	if err != nil {
	// 		log.Println("Error when processing bridge state: ", err)
	// 		bridgeManager, err = bridgeagg.InitStateFromDB(beaconFeatureStateDB)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 	}
	// }
	bridgeManagerJson := &jsonresult.BridgeAggState{
		BeaconTimeStamp:     blk.Header.Timestamp,
		UnifiedTokenVaults:  beaconBestState.BridgeAggManager().State().UnifiedTokenVaults(),
		WaitingUnshieldReqs: beaconBestState.BridgeAggManager().State().CloneWaitingUnshieldReqs(),
		BaseDecimal:         config.Param().BridgeAggParam.BaseDecimal,
		MaxLenOfPath:        config.Param().BridgeAggParam.MaxLenOfPath,
	}
	bridgeStr, err := json.MarshalToString(bridgeManagerJson)
	if err != nil {
		log.Println(err)
	}
	err = database.DBSaveBridgeState(bridgeStr, height, 1)
	if err != nil {
		log.Println(err)
	}

	// Process PDEstatev1
	if height < config.Param().PDexParams.Pdexv3BreakPointHeight {
		willProcess := true
		if blk.GetHeight() <= Localnode.GetBlockchain().GetCurrentBeaconBlockHeight(0)-50 {
			willProcess = false
		}
		if willProcess {
			// beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false, false)
			state := Localnode.GetBlockchain().GetBeaconBestState().PdeState(1)
			tradingFees := state.Reader().TradingFees()
			shares := state.Reader().Shares()

			waitingContributions := map[string]*rawdbv2.PDEContribution{}
			poolPairs := map[string]*rawdbv2.PDEPoolForPair{}

			err = json.Unmarshal(state.Reader().PoolPairs(), &poolPairs)
			if err != nil {
				panic(err)
			}

			err = json.Unmarshal(state.Reader().WaitingContributions(), &waitingContributions)
			if err != nil {
				panic(err)
			}

			pdeStateJSON := jsonresult.CurrentPDEState{
				BeaconTimeStamp:         beaconBestState.BestBlock.Header.Timestamp,
				PDEPoolPairs:            poolPairs,
				PDEShares:               shares,
				WaitingPDEContributions: waitingContributions,
				PDETradingFees:          tradingFees,
			}
			log.Printf("prepare pdeStateJSON for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
			pdeStr, err := json.MarshalToString(pdeStateJSON)
			if err != nil {
				log.Println(err)
			}
			err = database.DBSavePDEState(pdeStr, height, 1)
			if err != nil {
				log.Println(err)
			}
		}

		log.Printf("mongo stored pdeStateJSON for block %v beacon in %v\n", blk.GetHeight(), time.Since(startTime))
	}

	//process PDEstateV2
	if blk.GetHeight() >= config.Param().PDexParams.Pdexv3BreakPointHeight {
		// beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash
		// beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
		var prevStateV2 *shared.PDEStateV2
		stateV2 := &shared.PDEStateV2{
			StakingPoolsState: make(map[string]*pdex.StakingPoolState),
		}
		pdeStateJSON := jsonresult.Pdexv3State{}
		var pdeStr string
		var wg sync.WaitGroup
		if height > config.Param().PDexParams.Pdexv3BreakPointHeight {
			prevStateV2 = &shared.PDEStateV2{
				StakingPoolsState: make(map[string]*pdex.StakingPoolState),
			}
			if height > config.Param().PDexParams.Pdexv3BreakPointHeight+10 {
				if pdexV3State == nil {
					prevBeaconFeatureStateDB, err := Localnode.GetBlockchain().GetBestStateBeaconFeatureStateDBByHeight(height-1, Localnode.GetBlockchain().GetBeaconChainDatabase())
					if err != nil {
						log.Println(err)
					}
					params, err := statedb.GetPdexv3Params(prevBeaconFeatureStateDB)
					if err != nil {
						panic(err)
					}
					for stakingPoolID := range params.StakingPoolsShare() {
						stakers, liquidity, err := initStakers(stakingPoolID, prevBeaconFeatureStateDB)
						if err != nil {
							panic(err)
						}
						rewardsPerShare, err := statedb.GetPdexv3StakingPoolRewardsPerShare(prevBeaconFeatureStateDB, stakingPoolID)
						if err != nil {
							panic(err)
						}
						prevStateV2.StakingPoolsState[stakingPoolID] = pdex.NewStakingPoolStateWithValue(liquidity, stakers, rewardsPerShare)
					}
					poolPairs, err := initPoolPairStatesFromDB(prevBeaconFeatureStateDB)
					if err != nil {
						panic(err)
					}
					pools := make(map[string]*shared.PoolPairState)
					d, err := json.Marshal(poolPairs)
					if err != nil {
						panic(err)
					}
					err = json.Unmarshal(d, &pools)
					if err != nil {
						panic(err)
					}
					prevStateV2.PoolPairs = pools
				} else {
					prevPdexv3State := pdexV3State.Clone()
					pools := make(map[string]*shared.PoolPairState)
					err = json.Unmarshal(prevPdexv3State.Reader().PoolPairs(), &pools)
					if err != nil {
						panic(err)
					}
					prevStateV2.PoolPairs = pools
					prevStateV2.StakingPoolsState = prevPdexv3State.Reader().StakingPools()
				}
			}

		}
		if pdexV3State == nil {
			pdeStates, err := pdex.InitStatesFromDB(beaconFeatureStateDB, blk.GetHeight())
			if err != nil {
				panic(err)
			}
			pdexV3State = pdeStates[2]
		} else {
			pdeStateEnv := pdex.
				NewStateEnvBuilder().
				BuildBeaconInstructions(blk.Body.Instructions).
				BuildStateDB(beaconFeatureStateDB).
				BuildPrevBeaconHeight(blk.Header.Height - 1).
				BuildBCHeightBreakPointPrivacyV2(config.Param().BCHeightBreakPointPrivacyV2).
				BuildPdexv3BreakPoint(config.Param().PDexParams.Pdexv3BreakPointHeight).
				Build()
			err = pdexV3State.Process(pdeStateEnv)
			if err != nil {
				panic(err)
			}
			pdexV3State.ClearCache()
		}

		params := pdexV3State.Reader().Params()
		stateV2.StakingPoolsState = pdexV3State.Reader().StakingPools()
		pools := make(map[string]*shared.PoolPairState)
		err = json.Unmarshal(pdexV3State.Reader().PoolPairs(), &pools)
		if err != nil {
			panic(err)
		}
		stateV2.PoolPairs = pools
		stateV2.Params = params

		poolPairsJSON := make(map[string]*pdex.PoolPairState)
		err = json.Unmarshal(pdexV3State.Reader().PoolPairs(), &poolPairsJSON)
		if err != nil {
			panic(err)
		}

		wg.Add(2)
		go func() {
			paramJSON := pdexV3State.Reader().Params().Clone()
			waitingContributions := map[string]*rawdbv2.Pdexv3Contribution{}
			err = json.Unmarshal(pdexV3State.Reader().WaitingContributions(), &waitingContributions)
			if err != nil {
				panic(err)
			}
			pdeStateJSON = jsonresult.Pdexv3State{
				BeaconTimeStamp:      blk.Header.Timestamp,
				PoolPairs:            &poolPairsJSON,
				StakingPools:         &stateV2.StakingPoolsState,
				Params:               paramJSON,
				WaitingContributions: &waitingContributions,
			}
			pdeStr, err = json.MarshalToString(pdeStateJSON)
			if err != nil {
				log.Println(err)
			}
			wg.Done()
		}()

		var instructions []shared.InstructionBeaconData
		go func() {
			log.Printf("extractBeaconInstruction %v \n", blk.GetHeight())
			instructions, err = extractBeaconInstruction(blk.Body.Instructions)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		wg.Wait()

		log.Printf("prepare state beacon %v in %v\n", blk.GetHeight(), time.Since(startTime))
		willProcess := true

		if blk.GetHeight() <= Localnode.GetBlockchain().GetCurrentBeaconBlockHeight(0)-2 {
			willProcess = false
		}
		pairDatas, poolDatas, sharesDatas, poolStakeDatas, poolStakersDatas, orderBook, poolDatasToBeDel, sharesDatasToBeDel, poolStakeDatasToBeDel, poolStakersDatasToBeDel, orderBookToBeDel, rewardRecords, err := processPoolPairs(stateV2, prevStateV2, &pdeStateJSON, blk.GetHeight(), willProcess)
		if err != nil {
			panic(err)
		}
		wg.Add(15)
		go func() {
			defer wg.Done()
			log.Println("process tokens price")
			pools, err := database.DBGetDefaultPool(true)
			if err != nil {
				log.Println("PDEX token price", "DBGetDefaultPool", err)
				return
			}
			tokensMap := make(map[string]bool)
			for pool := range pools {
				tokens := strings.Split(pool, "-")
				tokensMap[tokens[0]] = true
				tokensMap[tokens[1]] = true
			}
			stableTokens, err := database.DBGetStableCoinID()
			if err != nil {
				log.Println("PDEX token price", "DBGetStableCoinID", err)
				return
			}
			tokens := make([]string, 0)
			for token := range tokensMap {
				isStable := false
				for _, v := range stableTokens {
					if strings.EqualFold(token, v) {
						isStable = true
						break
					}
				}
				if !isStable {
					tokens = append(tokens, token)
				}
			}
			baseToken, err := database.DBGetBasePriceToken()
			if err != nil {
				log.Println("PDEX token price", "DBGetBasePriceToken", err)
				return
			}
			var poolPairsArr []*shared.Pdexv3PoolPairWithId

			for poolId, element := range stateV2.PoolPairs {
				if _, ok := pools[poolId]; !ok {
					delete(*pdeStateJSON.PoolPairs, poolId)
					continue
				}
				var poolPair rawdbv2.Pdexv3PoolPair
				var poolPairWithId shared.Pdexv3PoolPairWithId

				poolPair = element.State
				poolPairWithId = shared.Pdexv3PoolPairWithId{
					poolPair,
					shared.Pdexv3PoolPairChild{
						PoolID: poolId},
				}
				poolPairsArr = append(poolPairsArr, &poolPairWithId)
			}
			var tokensPdexPrice []shared.TokenPdexPriceRecord
			for _, token := range tokens {
				tkInfo, err := database.DBGetExtraTokenInfoByTokenID([]string{token})
				if err != nil {
					log.Println("PDEX token price", "DBGetExtraTokenInfoByTokenID", err)
					continue
				}
				var price float64
				if token == common.PRVCoinID.String() {
					price = getPRVPrice(*pdeStateJSON.PoolPairs, baseToken)
				} else {
					price = getRateMinimum(token, baseToken, uint64(math.Pow10(int(tkInfo[0].PDecimals-6))), poolPairsArr, *pdeStateJSON.PoolPairs, pdeStateJSON.Params.DefaultFeeRateBPS)
				}
				newRecord := shared.TokenPdexPriceRecord{
					TokenID:      token,
					Price:        fmt.Sprintf("%f", price),
					BeaconHeight: blk.GetHeight(),
					BPToken:      baseToken,
				}
				tokensPdexPrice = append(tokensPdexPrice, newRecord)
			}
			if len(tokensPdexPrice) == 0 {
				log.Println("DBSaveTokensPdexPrice success")
				return
			}
			err = database.DBSaveTokensPdexPrice(tokensPdexPrice)
			if err != nil {
				log.Println("DBSaveTokensPdexPrice", err)
				return
			}
			log.Println("DBSaveTokensPdexPrice success")
		}()
		go func() {
			log.Printf("done process state beacon %v in %v %v \n", blk.GetHeight(), time.Since(startTime), willProcess)
			err = database.DBSavePDEState(pdeStr, height, 2)
			if err != nil {
				log.Println(err)
			}
			log.Printf("save pdex state beacon %v in %v\n", blk.GetHeight(), time.Since(startTime))

			wg.Done()
		}()

		go func() {
			err = database.DBSaveInstructionBeacon(instructions)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()

		go func() {
			err = database.DBSaveRewardRecord(rewardRecords)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()

		go func() {
			err = database.DBUpdatePDEPairListData(pairDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolInfoData(poolDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolShareData(sharesDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolStakeData(poolStakeDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdatePDEPoolStakerData(poolStakersDatas)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBUpdateOrderProgress(orderBook)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolData(poolDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolShareData(sharesDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolStakeData(poolStakeDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeletePDEPoolStakerData(poolStakersDatasToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		go func() {
			err = database.DBDeleteOrderProgress(orderBookToBeDel)
			if err != nil {
				panic(err)
			}
			wg.Done()
		}()
		wg.Wait()
		log.Printf("save pdex state 11 beacon %v in %v\n", blk.GetHeight(), time.Since(startTime))
	}

	statePrefix := BeaconData
	t10 := time.Now()
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	log.Printf("lvdb stored coin for block beacon %v in %v\n", blk.GetHeight(), time.Since(t10))
	currentState.blockProcessedLock.Lock()
	currentState.BlockProcessed[-1] = blk.Header.Height
	currentState.blockProcessedLock.Unlock()
	err = updateState()
	if err != nil {
		panic(err)
	}
	log.Printf("finish processing coin for block beacon %v in %v\n", blk.GetHeight(), time.Since(startTime))
}

func processPoolPairs(statev2 *shared.PDEStateV2, prevStatev2 *shared.PDEStateV2, stateV2Json *jsonresult.Pdexv3State, beaconHeight uint64, willProcess bool) ([]shared.PairInfoData, []shared.PoolInfoData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.PoolInfoData, []shared.PoolShareData, []shared.PoolStakeData, []shared.PoolStakerData, []shared.LimitOrderStatus, []shared.RewardRecord, error) {
	var pairList []shared.PairInfoData
	pairListMap := make(map[string][]shared.PoolInfoData)
	var poolPairs []shared.PoolInfoData
	var poolShare []shared.PoolShareData
	var stakePools []shared.PoolStakeData
	var poolStaking []shared.PoolStakerData
	var orderStatus []shared.LimitOrderStatus
	var rewardRecords []shared.RewardRecord

	var poolPairsToBeDelete []shared.PoolInfoData
	var poolShareToBeDelete []shared.PoolShareData
	var stakePoolsToBeDelete []shared.PoolStakeData
	var poolStakingToBeDelete []shared.PoolStakerData
	var orderStatusToBeDelete []shared.LimitOrderStatus
	if !willProcess {
		return pairList, poolPairs, poolShare, stakePools, poolStaking, orderStatus, poolPairsToBeDelete, poolShareToBeDelete, stakePoolsToBeDelete, poolStakingToBeDelete, orderStatusToBeDelete, rewardRecords, nil
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for poolID, state := range statev2.PoolPairs {
			poolData := shared.PoolInfoData{
				Version:        2,
				PoolID:         poolID,
				PairID:         state.State.Token0ID().String() + "-" + state.State.Token1ID().String(),
				AMP:            state.State.Amplifier(),
				TokenID1:       state.State.Token0ID().String(),
				TokenID2:       state.State.Token1ID().String(),
				Token1Amount:   fmt.Sprintf("%v", state.State.Token0RealAmount()),
				Token2Amount:   fmt.Sprintf("%v", state.State.Token1RealAmount()),
				Virtual1Amount: fmt.Sprintf("%v", state.State.Token0VirtualAmount().Uint64()),
				Virtual2Amount: fmt.Sprintf("%v", state.State.Token1VirtualAmount().Uint64()),
				TotalShare:     fmt.Sprintf("%v", state.State.ShareAmount()),
			}
			poolPairs = append(poolPairs, poolData)
			pairListMap[poolData.PairID] = append(pairListMap[poolData.PairID], poolData)
			collectedShared := make(map[string]struct{})
			for shareID, share := range state.Shares {
				tradingFee := make(map[string]uint64)
				orderReward := make(map[string]uint64)
				shareIDHash, err := common.Hash{}.NewHashFromStr(shareID)
				if err != nil {
					panic(err)
				}
				rewards, err := recomputeLPFee(state.Shares, state.LpFeesPerShare, state.LmRewardsPerShare, *shareIDHash)
				if err != nil {
					panic(err)
				}
				if _, ok := state.OrderRewards[shareID]; ok {
					for tokenID, amount := range state.OrderRewards[shareID].UncollectedRewards {
						orderReward[tokenID.String()] = amount
					}
					collectedShared[shareID] = struct{}{}
				}
				for k, v := range rewards {
					tradingFee[k.String()] = v
				}
				shareData := shared.PoolShareData{
					Version:     2,
					PoolID:      poolID,
					Amount:      share.Amount(),
					TradingFee:  tradingFee,
					OrderReward: orderReward,
					NFTID:       shareID,
				}
				poolShare = append(poolShare, shareData)
			}

			for shareID, share := range state.OrderRewards {
				orderReward := make(map[string]uint64)
				if _, ok := collectedShared[shareID]; !ok {
					for tokenID, amount := range share.UncollectedRewards {
						orderReward[tokenID.String()] = amount
					}
					shareData := shared.PoolShareData{
						Version:     2,
						PoolID:      poolID,
						OrderReward: orderReward,
						NFTID:       shareID,
					}
					poolShare = append(poolShare, shareData)
				}
			}

			for _, order := range state.Orderbook.Orders {
				newOrder := shared.LimitOrderStatus{
					RequestTx:     order.Id(),
					Token1Balance: fmt.Sprintf("%v", order.Token0Balance()),
					Token2Balance: fmt.Sprintf("%v", order.Token1Balance()),
					Direction:     order.TradeDirection(),
					PoolID:        poolID,
					PairID:        poolData.PairID,
					NftID:         order.NftID().String(),
				}
				orderStatus = append(orderStatus, newOrder)
			}
		}
		for pairID, pools := range pairListMap {
			data := shared.PairInfoData{
				PairID:    pairID,
				TokenID1:  pools[0].TokenID1,
				TokenID2:  pools[0].TokenID2,
				PoolCount: len(pools),
			}
			tk1Amount := uint64(0)
			tk2Amount := uint64(0)
			for _, v := range pools {
				a1, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
				a2, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
				tk1Amount += a1
				tk2Amount += a2
			}
			data.Token1Amount = fmt.Sprintf("%v", tk1Amount)
			data.Token2Amount = fmt.Sprintf("%v", tk2Amount)
			pairList = append(pairList, data)
		}
		wg.Done()
	}()

	go func() {
		for tokenID, stakeData := range statev2.StakingPoolsState {
			poolData := shared.PoolStakeData{
				Amount:  stakeData.Liquidity(),
				TokenID: tokenID,
			}
			stakePools = append(stakePools, poolData)
			for shareID, staker := range stakeData.Stakers() {
				rewardMap := make(map[string]uint64)

				shareIDHash, err := common.Hash{}.NewHashFromStr(shareID)
				if err != nil {
					panic(err)
				}
				reward, err := statev2.StakingPoolsState[tokenID].RecomputeStakingRewards(*shareIDHash)
				if err != nil {
					panic(err)
				}
				for k, v := range reward {
					rewardMap[k.String()] = v
				}
				stake := shared.PoolStakerData{
					TokenID: tokenID,
					NFTID:   shareID,
					Amount:  staker.Liquidity(),
					Reward:  rewardMap,
				}
				poolStaking = append(poolStaking, stake)
			}
		}
		wg.Done()
	}()
	wg.Wait()
	for tokenID, _ := range statev2.StakingPoolsState {
		willDelete := false
		if _, ok := statev2.Params.StakingPoolsShare[tokenID]; !ok {
			willDelete = true
		}
		if willDelete {
			poolData := shared.PoolStakeData{
				TokenID: tokenID,
			}
			stakePoolsToBeDelete = append(stakePoolsToBeDelete, poolData)
		}
	}
	//comparing with old state
	if prevStatev2 != nil {
		var wg sync.WaitGroup
		var poolPairsArr []*shared.Pdexv3PoolPairWithId
		for poolId, element := range statev2.PoolPairs {

			var poolPair rawdbv2.Pdexv3PoolPair
			var poolPairWithId shared.Pdexv3PoolPairWithId

			poolPair = element.State
			poolPairWithId = shared.Pdexv3PoolPairWithId{
				poolPair,
				shared.Pdexv3PoolPairChild{
					PoolID: poolId},
			}
			if poolPair.ShareAmount() == 0 {
				continue
			}
			tk1, err := database.DBGetExtraTokenInfo(poolPair.Token0ID().String())
			if err != nil {
				log.Println(err)
			}
			tk2, err := database.DBGetExtraTokenInfo(poolPair.Token1ID().String())
			if err != nil {
				log.Println(err)
			}
			tk1Decimal := 9
			tk2Decimal := 9
			if tk1 != nil {
				tk1Decimal = int(tk1.PDecimals)
			}
			if tk2 != nil {
				tk2Decimal = int(tk2.PDecimals)
			}
			if poolPair.Token0RealAmount() < uint64(math.Pow10(tk1Decimal-3)) || poolPair.Token1RealAmount() < uint64(math.Pow10(tk2Decimal-3)) {
				continue
			}
			poolPairsArr = append(poolPairsArr, &poolPairWithId)
		}
		wg.Add(3)
		go func() {

			defaultPools, err := database.DBGetDefaultPool(true)
			if err != nil {
				log.Println("database.DBGetDefaultPool", err)
			}
			var newPools []*shared.Pdexv3PoolPairWithId
			for _, pool := range poolPairsArr {
				if _, ok := defaultPools[pool.PoolID]; ok {
					newPools = append(newPools, pool)
				} else {
					delete(*stateV2Json.PoolPairs, pool.PoolID)
				}
			}
			if len(defaultPools) > 0 {
				poolPairsArr = newPools
			}

			for tokenID, state := range statev2.StakingPoolsState {
				rw, err := extractPDEStakingReward(tokenID, statev2.StakingPoolsState, prevStatev2.StakingPoolsState, beaconHeight)
				if err != nil {
					panic(err)
				}
				rewardReceive := uint64(0)
				tokenStakeAmount := state.Liquidity()
				receiveStake := uint64(0)

				if tokenID == common.PRVCoinID.String() {
					receiveStake = tokenStakeAmount
				} else {
					tkInfo, err := database.DBGetExtraTokenInfo(tokenID)
					if err != nil {
						log.Println(err)
					}
					tkDecimal := 1
					if tkInfo != nil {
						tkDecimal = int(tkInfo.PDecimals)
					}
					rateRw := getRateMinimum(tokenID, common.PRVCoinID.String(), uint64(math.Pow10(tkDecimal)), poolPairsArr, *stateV2Json.PoolPairs, stateV2Json.Params.DefaultFeeRateBPS)

					receiveStake = uint64(float64(tokenStakeAmount) * rateRw)
				}

				for tk, v := range rw {
					if tk == common.PRVCoinID.String() {
						rewardReceive += v
						continue
					}
					tkInfo, err := database.DBGetExtraTokenInfo(tk)
					if err != nil {
						log.Println(err)
					}
					tkRwDecimal := 1
					if tkInfo != nil {
						tkRwDecimal = int(tkInfo.PDecimals)
					}
					rateRw := getRateMinimum(tk, common.PRVCoinID.String(), uint64(math.Pow10(tkRwDecimal)), poolPairsArr, *stateV2Json.PoolPairs, stateV2Json.Params.DefaultFeeRateBPS)

					receive := uint64(float64(v) * rateRw)
					rewardReceive += receive
				}
				var rwInfo struct {
					RewardPerToken     map[string]uint64
					TokenAmount        map[string]uint64
					RewardReceiveInPRV uint64
					TotalAmountInPRV   uint64
				}
				rwInfo.TokenAmount = make(map[string]uint64)
				rwInfo.RewardPerToken = rw
				rwInfo.TokenAmount[tokenID] = tokenStakeAmount
				rwInfo.RewardReceiveInPRV = rewardReceive
				rwInfo.TotalAmountInPRV = receiveStake

				rwInfoBytes, err := json.Marshal(rwInfo)
				if err != nil {
					panic(err)
				}
				data := shared.RewardRecord{
					DataID:       tokenID,
					Data:         string(rwInfoBytes),
					BeaconHeight: beaconHeight,
				}
				rewardRecords = append(rewardRecords, data)
			}
			for poolID, state := range statev2.PoolPairs {
				rewardReceive := uint64(0)
				token1Amount := state.State.Token0RealAmount()
				token2Amount := state.State.Token1RealAmount()
				total1 := uint64(0)
				total2 := uint64(0)
				if token1Amount == 0 || token2Amount == 0 {
					continue
				}
				rw, err := extractLqReward(poolID, statev2.PoolPairs, prevStatev2.PoolPairs)
				if err != nil {
					panic(err)
				}
				if state.State.Token0ID().String() == common.PRVCoinID.String() || state.State.Token1ID().String() == common.PRVCoinID.String() {
					if state.State.Token0ID().String() == common.PRVCoinID.String() {
						total1 = token1Amount
						total2 = token1Amount
					} else {
						total1 = token2Amount
						total2 = token2Amount
					}
				} else {
					tk1, err := database.DBGetExtraTokenInfo(state.State.Token0ID().String())
					if err != nil {
						log.Println(err)
					}
					tk2, err := database.DBGetExtraTokenInfo(state.State.Token1ID().String())
					if err != nil {
						log.Println(err)
					}
					tk1Decimal := 9
					tk2Decimal := 9
					if tk1 != nil {
						tk1Decimal = int(tk1.PDecimals)
					}
					if tk2 != nil {
						tk2Decimal = int(tk2.PDecimals)
					}
					if token1Amount < uint64(math.Pow10(tk1Decimal-3)) || token2Amount < uint64(math.Pow10(tk2Decimal-3)) {
						continue
					}
					rate1 := getRateMinimum(state.State.Token0ID().String(), common.PRVCoinID.String(), uint64(math.Pow10(tk1Decimal)), poolPairsArr, *stateV2Json.PoolPairs, stateV2Json.Params.DefaultFeeRateBPS)

					rate2 := getRateMinimum(state.State.Token1ID().String(), common.PRVCoinID.String(), uint64(math.Pow10(tk2Decimal)), poolPairsArr, *stateV2Json.PoolPairs, stateV2Json.Params.DefaultFeeRateBPS)

					total1 = uint64(float64(token1Amount) * rate1)
					total2 = uint64(float64(token2Amount) * rate2)
				}

				totalAmount := total1 + total2
				for tk, v := range rw {
					if tk == common.PRVCoinID.String() {
						rewardReceive += v
						continue
					}
					tkInfo, err := database.DBGetExtraTokenInfo(tk)
					if err != nil {
						log.Println(err)
					}
					tkRwDecimal := 9
					if tkInfo != nil {
						tkRwDecimal = int(tkInfo.PDecimals)
					}
					rateRw := getRateMinimum(tk, common.PRVCoinID.String(), uint64(math.Pow10(tkRwDecimal)), poolPairsArr, *stateV2Json.PoolPairs, stateV2Json.Params.DefaultFeeRateBPS)

					receive := uint64(float64(v) * rateRw)
					rewardReceive += receive
				}
				var rwInfo struct {
					RewardPerToken     map[string]uint64
					TokenAmount        map[string]uint64
					RewardReceiveInPRV uint64
					TotalAmountInPRV   uint64
				}
				rwInfo.TokenAmount = make(map[string]uint64)
				rwInfo.RewardPerToken = rw
				rwInfo.TokenAmount[state.State.Token0ID().String()] = token1Amount
				rwInfo.TokenAmount[state.State.Token1ID().String()] = token2Amount
				rwInfo.RewardReceiveInPRV = rewardReceive
				rwInfo.TotalAmountInPRV = totalAmount

				rwInfoBytes, err := json.Marshal(rwInfo)
				if err != nil {
					panic(err)
				}
				data := shared.RewardRecord{
					DataID:       poolID,
					Data:         string(rwInfoBytes),
					BeaconHeight: beaconHeight,
				}
				rewardRecords = append(rewardRecords, data)
			}
			wg.Done()
		}()

		go func() {
			for poolID, state := range prevStatev2.PoolPairs {
				willDelete := false
				if _, ok := statev2.PoolPairs[poolID]; !ok {
					willDelete = true
				}
				if willDelete {
					tkState := state.State
					poolData := shared.PoolInfoData{
						Version: 2,
						PoolID:  poolID,
						PairID:  tkState.Token0ID().String() + "-" + tkState.Token1ID().String(),
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
							if _, ok2 := newState.OrderRewards[shareID]; !ok2 {
								willDelete = true
							}
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
			wg.Done()
		}()

		go func() {
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
			wg.Done()
		}()
		wg.Wait()
	}
	return pairList, poolPairs, poolShare, stakePools, poolStaking, orderStatus, poolPairsToBeDelete, poolShareToBeDelete, stakePoolsToBeDelete, poolStakingToBeDelete, orderStatusToBeDelete, rewardRecords, nil
}

func extractBeaconInstruction(insts [][]string) ([]shared.InstructionBeaconData, error) {
	var result []shared.InstructionBeaconData
	for _, inst := range insts {
		data := shared.InstructionBeaconData{}
		if len(inst) <= 2 {
			continue
		}
		metadataType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue // Not error, just not PDE instructions
		}

		data.Metatype = inst[0]
		switch metadataType {
		case metadata.PDEWithdrawalRequestMeta:
			if inst[2] == common.PDEWithdrawalAcceptedChainStatus {
				contentBytes := []byte(inst[3])
				var withdrawalRequestAction metadata.PDEWithdrawalRequestAction
				err = json.Unmarshal(contentBytes, &withdrawalRequestAction)
				if err != nil {
					panic(err)
				}
				data.Content = inst[3]
				data.Status = inst[2]
				data.TxRequest = withdrawalRequestAction.TxReqID.String()
			}

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

		case metadataCommon.Pdexv3StakingRequestMeta:
			data.Status = inst[1]
			data.Content = inst[2]
			switch inst[1] {
			case common.Pdexv3AcceptUnstakingStatus:
				acceptInst := instruction.NewAcceptStaking()
				err := acceptInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = acceptInst.TxReqID().String()
			case common.Pdexv3RejectUnstakingStatus:
				rejectInst := instruction.NewRejectStaking()
				err := rejectInst.FromStringSlice(inst)
				if err != nil {
					panic(err)
				}
				data.TxRequest = rejectInst.TxReqID().String()
			}
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
		case metadataCommon.IssuingResponseMeta:
			//TODO: add shield respond
		default:
			continue
		}

		result = append(result, data)
	}
	return result, nil
}

func extractLqReward(poolID string, curPools map[string]*shared.PoolPairState, prevPools map[string]*shared.PoolPairState) (map[string]uint64, error) {
	result := make(map[string]uint64)

	curLPFeesPerShare, curLMFeesPerShare, shareAmount, LmLockedShareAmount, err := getLPFeesPerShare(poolID, curPools)
	if err != nil {
		return nil, err
	}

	oldLPFeesPerShare, oldLMFeesPerShare, _, _, err := getLPFeesPerShare(poolID, prevPools)
	if err != nil {
		oldLPFeesPerShare = map[common.Hash]*big.Int{}
	}

	for tokenID := range curLPFeesPerShare {
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(shareAmount))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID.String()] = reward.Uint64()
		}
	}

	for tokenID := range curLMFeesPerShare {
		oldFees, isExisted := oldLMFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLMFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(shareAmount-LmLockedShareAmount))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			if _, ok := result[tokenID.String()]; ok {
				result[tokenID.String()] += reward.Uint64()
			} else {

				result[tokenID.String()] = reward.Uint64()
			}
		}
	}

	return result, nil
}

func getLPFeesPerShare(pairID string, pools map[string]*shared.PoolPairState) (map[common.Hash]*big.Int, map[common.Hash]*big.Int, uint64, uint64, error) {
	if _, ok := pools[pairID]; !ok {
		return nil, nil, 0, 0, fmt.Errorf("Pool pair %s not found", pairID)
	}
	pair := pools[pairID]
	return pair.LpFeesPerShare, pair.LmRewardsPerShare, pair.State.ShareAmount(), pair.State.LmLockedShareAmount(), nil
}

func getStakingRewardsPerShare(
	stakingPoolID string, pools map[string]*pdex.StakingPoolState,
) (map[common.Hash]*big.Int, uint64, error) {

	if _, ok := pools[stakingPoolID]; !ok {
		return nil, 0, fmt.Errorf("Staking pool %s not found", stakingPoolID)
	}

	return pools[stakingPoolID].RewardsPerShare(), pools[stakingPoolID].Liquidity(), nil
}

func extractPDEStakingReward(poolID string, curPools map[string]*pdex.StakingPoolState, prevPools map[string]*pdex.StakingPoolState, beaconHeight uint64) (map[string]uint64, error) {
	result := make(map[string]uint64)

	curLPFeesPerShare, shareAmount, err := getStakingRewardsPerShare(poolID, curPools)
	if err != nil {
		return nil, err
	}

	oldLPFeesPerShare, _, err := getStakingRewardsPerShare(poolID, prevPools)
	if err != nil {
		oldLPFeesPerShare = map[common.Hash]*big.Int{}
	}

	for tokenID := range curLPFeesPerShare {
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(shareAmount))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID.String()] = reward.Uint64()
		}
	}

	return result, nil
}
