package chainsynker

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

func GetOTACoinsByIndices(shardID byte, tokenID common.Hash, idxList []uint64) (map[uint64]jsonresult.ICoinInfo, error) {
	txDb := Localnode.GetBlockchain().GetBestStateShard(shardID).GetCopiedTransactionStateDB()
	res := make(map[uint64]jsonresult.ICoinInfo)
	for _, idx := range idxList {
		coinBytes, err := statedb.GetOTACoinByIndex(txDb, tokenID, idx, shardID)
		if err != nil {
			return nil, err
		}

		tmpCoin := new(coin.CoinV2)
		err = tmpCoin.SetBytes(coinBytes)
		if err != nil {
			return nil, err
		}

		res[idx] = tmpCoin
	}

	return res, nil
}

func GetOTACoinLength() (map[string]map[byte]uint64, error) {
	res := make(map[string]map[byte]uint64)
	prvLength := make(map[byte]uint64)
	tokenLength := make(map[byte]uint64)
	for shardID := byte(0); shardID < byte(common.MaxShardNumber); shardID++ {
		txDb := Localnode.GetBlockchain().GetBestStateShard(shardID).GetCopiedTransactionStateDB()
		shardPRVLength, err := statedb.GetOTACoinLength(txDb, common.PRVCoinID, shardID)
		if err != nil {
			return nil, err
		}
		prvLength[shardID] = shardPRVLength.Uint64()

		shardTokenLength, err := statedb.GetOTACoinLength(txDb, common.ConfidentialAssetID, shardID)
		if err != nil {
			return nil, err
		}
		tokenLength[shardID] = shardTokenLength.Uint64()
	}
	res[common.PRVCoinID.String()] = prvLength
	res[common.ConfidentialAssetID.String()] = tokenLength

	return res, nil
}
