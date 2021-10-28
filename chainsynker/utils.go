package chainsynker

import (
	"fmt"
	"math/big"

	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func initStakers(stakingPoolID string, stateDB *statedb.StateDB) (map[string]*pdex.Staker, uint64, error) {
	res := make(map[string]*pdex.Staker)
	totalLiquidity := uint64(0)
	stakerStates, err := statedb.GetPdexv3Stakers(stateDB, stakingPoolID)
	if err != nil {
		return res, totalLiquidity, err
	}
	for nftID, stakerState := range stakerStates {
		totalLiquidity += stakerState.Liquidity()
		rewards, err := statedb.GetPdexv3StakerRewards(stateDB, stakingPoolID, nftID)
		if err != nil {
			return res, totalLiquidity, err
		}
		lastRewardsPerShare, err := statedb.GetPdexv3StakerLastRewardsPerShare(stateDB, stakingPoolID, nftID)
		if err != nil {
			return res, totalLiquidity, err
		}
		res[nftID] = pdex.NewStakerWithValue(stakerState.Liquidity(), rewards, lastRewardsPerShare)
	}
	return res, totalLiquidity, nil
}

func initPoolPairStatesFromDB(stateDB *statedb.StateDB) (map[string]*pdex.PoolPairState, error) {
	poolPairsStates, err := statedb.GetPdexv3PoolPairs(stateDB)
	if err != nil {
		return nil, err
	}
	res := make(map[string]*pdex.PoolPairState)
	for poolPairID, poolPairState := range poolPairsStates {
		lpFeesPerShare, err := statedb.GetPdexv3PoolPairLpFeesPerShares(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		protocolFees, err := statedb.GetPdexv3PoolPairProtocolFees(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		stakingPoolFees, err := statedb.GetPdexv3PoolPairStakingPoolFees(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		shares, err := initShares(poolPairID, stateDB)
		if err != nil {
			return nil, err
		}

		orderbook := &pdex.Orderbook{}
		orderMap, err := statedb.GetPdexv3Orders(stateDB, poolPairState.PoolPairID())
		if err != nil {
			return nil, err
		}
		for _, item := range orderMap {
			v := item.Value()
			orderbook.InsertOrder(&v)
		}
		poolPair := pdex.NewPoolPairStateWithValue(
			poolPairState.Value(), shares, *orderbook,
			lpFeesPerShare, protocolFees, stakingPoolFees,
		)
		res[poolPairID] = poolPair
	}
	return res, nil
}

func initShares(poolPairID string, stateDB *statedb.StateDB) (map[string]*pdex.Share, error) {
	res := make(map[string]*pdex.Share)
	shareStates, err := statedb.GetPdexv3Shares(stateDB, poolPairID)
	if err != nil {
		return nil, err
	}
	for nftID, shareState := range shareStates {
		tradingFees, err := statedb.GetPdexv3ShareTradingFees(stateDB, poolPairID, nftID)
		if err != nil {
			return nil, err
		}
		lastLPFeesPerShare, err := statedb.GetPdexv3ShareLastLpFeesPerShare(stateDB, poolPairID, nftID)
		if err != nil {
			return nil, err
		}
		res[nftID] = pdex.NewShareWithValue(shareState.Amount(), tradingFees, lastLPFeesPerShare)
	}
	return res, nil
}

func recomputeLPFee(
	shares map[string]*pdex.Share,
	lpFeesPerShare map[common.Hash]*big.Int,
	nftID common.Hash,
) (map[common.Hash]uint64, error) {
	result := map[common.Hash]uint64{}

	curShare, ok := shares[nftID.String()]
	if !ok {
		return nil, fmt.Errorf("Share not found")
	}

	curLPFeesPerShare := lpFeesPerShare
	oldLPFeesPerShare := curShare.LastLPFeesPerShare()

	for tokenID := range curLPFeesPerShare {
		tradingFee, isExisted := curShare.TradingFees()[tokenID]
		if !isExisted {
			tradingFee = 0
		}
		oldFees, isExisted := oldLPFeesPerShare[tokenID]
		if !isExisted {
			oldFees = big.NewInt(0)
		}
		newFees := curLPFeesPerShare[tokenID]

		reward := new(big.Int).Mul(new(big.Int).Sub(newFees, oldFees), new(big.Int).SetUint64(curShare.Amount()))
		reward = new(big.Int).Div(reward, pdex.BaseLPFeesPerShare)
		reward = new(big.Int).Add(reward, new(big.Int).SetUint64(tradingFee))

		if !reward.IsUint64() {
			return nil, fmt.Errorf("Reward of token %v is out of range", tokenID)
		}
		if reward.Uint64() > 0 {
			result[tokenID] = reward.Uint64()
		}
	}
	return result, nil
}
