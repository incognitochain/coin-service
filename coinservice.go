package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/syndtr/goleveldb/leveldb"
)

var localnode interface {
	GetUserDatabase() *leveldb.DB
	GetBlockchain() *blockchain.BlockChain
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
}

var stateLock sync.Mutex
var ShardProcessedState map[byte]uint64
var TransactionStateDB map[byte]*statedb.StateDB

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

	shardID := blk.GetShardID()
	stateLock.Lock()
	transactionStateDB := TransactionStateDB[byte(shardID)]
	stateLock.Unlock()

	if len(blk.Body.Transactions) > 0 {
		err = bc.CreateAndSaveTxViewPointFromBlock(&blk, transactionStateDB)
		if err != nil {
			panic(err)
		}
	}

	transactionRootHash, err := transactionStateDB.Commit(true)
	if err != nil {
		panic(err)
	}
	err = transactionStateDB.Database().TrieDB().Commit(transactionRootHash, false)
	if err != nil {
		panic(err)
	}
	bc.GetBestStateShard(byte(blk.GetShardID())).TransactionStateDBRootHash = transactionRootHash
	batchData := bc.GetShardChainDatabase(blk.Header.ShardID).NewBatch()
	err = bc.BackupShardViews(batchData, blk.Header.ShardID)
	if err != nil {
		panic("Backup shard view error")
	}

	if err := batchData.Write(); err != nil {
		panic(err)
	}
	// localnode.GetBlockchain().ShardChain[shardID] = blockchain.NewShardChain(shardID, multiview.NewMultiView(), localnode.GetBlockchain().GetConfig().BlockGen, localnode.GetBlockchain(), common.GetShardChainKey(blk.Header.ShardID))
	// if err := localnode.GetBlockchain().RestoreShardViews(blk.Header.ShardID); err != nil {
	// 	panic(err)
	// }
	// stateLock.Lock()
	// TransactionStateDB[byte(blk.GetShardID())] = localnode.GetBlockchain().GetBestStateShard(blk.Header.ShardID).GetCopiedTransactionStateDB()
	// stateLock.Unlock()

	//store output-coin and keyimage on db
	keyImageList := []KeyImageData{}
	outCoinList := []CoinData{}
	beaconHeight := blk.Header.BeaconHeight

	for _, tx := range blk.Body.Transactions {
		txHash := tx.Hash().String()
		tokenID := tx.GetTokenID().String()
		if tx.GetVersion() == 2 {
			if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
				fmt.Println("\n====================================================")
				fmt.Println(tokenID, txHash, tx.IsPrivacy(), tx.GetProof(), tx.GetVersion(), tx.GetMetadataType())
				ins := tx.GetProof().GetInputCoins()
				outs := tx.GetProof().GetOutputCoins()

				for _, coin := range ins {
					km := NewKeyImageData(tokenID, "", txHash, coin.GetKeyImage().ToBytesS(), beaconHeight, shardID)
					keyImageList = append(keyImageList, *km)
				}
				for _, coin := range outs {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {

							fmt.Println("len(outs))", len(outs), base58.Base58Check{}.Encode(publicKeyBytes, 0))
							panic(err)
						}
						outCoin := NewCoinData(beaconHeight, idxBig.Uint64(), coin.Bytes(), tokenID, coin.GetPublicKey().String(), "", txHash, shardID)
						outCoinList = append(outCoinList, *outCoin)
					}
				}
				fmt.Println(tokenID, txHash, len(ins), len(outs))
				fmt.Println("====================================================\n")
			}
			if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
				fmt.Println("\n====================================================")
				fmt.Println(common.ConfidentialAssetID.String(), txHash, tx.IsPrivacy(), tx.GetProof(), tx.GetVersion(), tx.GetMetadataType())
				txToken := tx.(transaction.TransactionToken)
				txTokenData := txToken.GetTxTokenData()
				tokenIns := txTokenData.TxNormal.GetProof().GetInputCoins()
				tokenOuts := txTokenData.TxNormal.GetProof().GetOutputCoins()
				for _, coin := range tokenIns {
					km := NewKeyImageData(common.ConfidentialAssetID.String(), "", txHash, coin.GetKeyImage().ToBytesS(), beaconHeight, shardID)
					keyImageList = append(keyImageList, *km)
				}
				for _, coin := range tokenOuts {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], *tx.GetTokenID(), publicKeyBytes)
						if err != nil {
							panic(err)
						}
						outCoin := NewCoinData(beaconHeight, idxBig.Uint64(), coin.Bytes(), common.ConfidentialAssetID.String(), coin.GetPublicKey().String(), "", txHash, shardID)
						outCoinList = append(outCoinList, *outCoin)
					}
				}
				fmt.Println(common.ConfidentialAssetID.String(), txHash, len(tokenIns), len(tokenOuts))
				fmt.Println("====================================================\n")
				if tx.GetTxFee() > 0 {
					ins := tx.GetProof().GetInputCoins()
					outs := tx.GetProof().GetOutputCoins()
					for _, coin := range ins {
						km := NewKeyImageData(common.PRVCoinID.String(), "", txHash, coin.GetKeyImage().ToBytesS(), beaconHeight, shardID)
						keyImageList = append(keyImageList, *km)
					}
					for _, coin := range outs {
						publicKeyBytes := coin.GetPublicKey().ToBytesS()
						publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
						if publicKeyShardID == byte(shardID) {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
							if err != nil {
								panic(err)
							}
							outCoin := NewCoinData(beaconHeight, idxBig.Uint64(), coin.Bytes(), common.PRVCoinID.String(), coin.GetPublicKey().String(), "", txHash, shardID)
							outCoinList = append(outCoinList, *outCoin)
						}
					}
				}
			}
		}

	}
	if len(outCoinList) > 0 {
		err = DBSaveCoins(outCoinList)
		if err != nil {
			panic(err)
		}
	}
	if len(keyImageList) > 0 {
		err = DBSaveUsedKeyimage(keyImageList)
		if err != nil {
			panic(err)
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
	log.Println("initiating coin-service...")
	ShardProcessedState = make(map[byte]uint64)
	TransactionStateDB = make(map[byte]*statedb.StateDB)
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
		TransactionStateDB[byte(i)] = localnode.GetBlockchain().GetBestStateShard(byte(i)).GetCopiedTransactionStateDB()
	}
	for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		localnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
}
