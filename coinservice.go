package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/syndtr/goleveldb/leveldb"
)

var localnode interface {
	GetUserDatabase() *leveldb.DB
	GetBlockchain() *blockchain.BlockChain
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
}

var stateLock sync.Mutex
var ShardProcessedState map[byte]uint64

func OnNewShardBlock(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	var blk blockchain.ShardBlock
	blkBytes, err := localnode.GetUserDatabase().Get(h.Bytes(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		fmt.Println(err)
		return
	}
	//store output-coin and keyimage on db

	for _, tx := range blk.Body.Transactions {
		if tx.GetType() == "n" || tx.GetType() == "tp" {
			txHash := tx.Hash().String()
			tokenID := tx.GetTokenID().String()

			fmt.Println(txHash, tokenID)
			// ins := tx.GetProof().GetInputCoins()
			// for _, coin := range ins {
			// 	coin.GetAssetTag().String()
			// }
		}
	}

	statePrefix := fmt.Sprintf("coin-processed-%v", blk.Header.ShardID)
	err = localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
	stateLock.Lock()
	ShardProcessedState[blk.Header.ShardID] = blk.Header.Height
	stateLock.Unlock()
}

func initCoinService() {
	ShardProcessedState = make(map[byte]uint64)
	//load ShardProcessedState
	for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		statePrefix := fmt.Sprintf("coin-processed-%v", i)
		v, err := localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
		if err != nil {
			fmt.Println(err)
		}
		if v != nil {
			height, err := strconv.ParseUint(string(v), 0, 64)
			if err != nil {
				fmt.Println(err)
				continue
			}
			ShardProcessedState[byte(i)] = height
		} else {
			ShardProcessedState[byte(i)] = 1
		}
	}
	for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		localnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
}
