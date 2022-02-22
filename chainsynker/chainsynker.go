package chainsynker

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/peerv2/proto"
	"github.com/incognitochain/incognito-chain/privacy"
	jsoniter "github.com/json-iterator/go"

	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	devframework "github.com/0xkumi/incognito-dev-framework"
	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/blockchain/types"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/kamva/mgm/v3"
	"github.com/syndtr/goleveldb/leveldb"
)

var json = jsoniter.ConfigFastest
var Localnode interface {
	OnReceive(msgType int, f func(msg interface{}))
	GetUserDatabase() *leveldb.DB
	GetBlockchain() *blockchain.BlockChain
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64, chainID int))
	GetShardState(shardID int) (uint64, *common.Hash)
	SyncSpecificShardBlockBytes(shardID int, height uint64, blockHash string) ([]byte, error)
}

// var ShardProcessedState map[byte]uint64
var TransactionStateDB map[byte]*statedb.StateDB
var blockProcessed map[int]uint64
var blockProcessedLock sync.RWMutex
var lastTokenIDMap map[string]string
var lastTokenIDLock sync.RWMutex
var chainDataFolder string
var useFullnodeData bool

func InitChainSynker(cfg shared.Config) {
	lastTokenIDMap = make(map[string]string)
	blockProcessed = make(map[int]uint64)
	highwayAddress := cfg.Highway
	chainDataFolder = cfg.ChainDataFolder
	useFullnodeData = cfg.FullnodeData
	if !cfg.FullnodeData {
		panic(8888)
	}
	if shared.RESET_FLAG {
		err := ResetMongoAndReSync()
		if err != nil {
			panic(err)
		}
	}

	err := database.DBCreateCoinV1Index()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateCoinV2Index()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateKeyimageIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateTxIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateTxPendingIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateTokenIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateProcessorIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateLiquidityIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateShieldIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateProcessorIndex()
	if err != nil {
		panic(err)
	}

	err = database.DBCreateTradeIndex()
	if err != nil {
		panic(err)
	}

	var netw devframework.NetworkParam
	netw.HighwayAddress = highwayAddress
	node := devframework.NewAppNode(chainDataFolder, netw, !useFullnodeData, false, false, cfg.EnableChainLog)
	Localnode = node
	log.Println("initiating chain-synker...")
	if shared.RESET_FLAG {
		for i := 0; i < Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
			statePrefix := fmt.Sprintf("%v%v", ShardData, i)
			err := Localnode.GetUserDatabase().Delete([]byte(statePrefix), nil)
			if err != nil {
				panic(err)
			}
		}
		beaconStatePrefix := BeaconData
		err := Localnode.GetUserDatabase().Delete([]byte(beaconStatePrefix), nil)
		if err != nil {
			panic(err)
		}
		log.Println("=========================")
		log.Println("RESET SUCCESS")
		log.Println("=========================")
	}
	ShardProcessedState := make(map[byte]uint64)
	TransactionStateDB = make(map[byte]*statedb.StateDB)
	ProcessedBeaconBestState := uint64(1)
	for i := 0; i < Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
		statePrefix := fmt.Sprintf("%v%v", ShardData, i)
		v, err := Localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
		if err != nil {
			log.Println(err)
			ShardProcessedState[byte(i)] = 1
		} else {
			height, err := strconv.ParseUint(string(v), 0, 64)
			if err != nil {
				panic(err)
			}
			ShardProcessedState[byte(i)] = height
		}
		if !useFullnodeData {
			TransactionStateDB[byte(i)] = Localnode.GetBlockchain().GetBestStateShard(byte(i)).GetCopiedTransactionStateDB()
		} else {
			TransactionStateDB[byte(i)] = Localnode.GetBlockchain().GetBestStateTransactionStateDB(byte(i))
		}

	}
	beaconStatePrefix := BeaconData
	v, err := Localnode.GetUserDatabase().Get([]byte(beaconStatePrefix), nil)
	if err != nil {
		log.Println(err)
	} else {
		height, err := strconv.ParseUint(string(v), 0, 64)
		if err != nil {
			panic(err)
		}
		ProcessedBeaconBestState = height
	}

	go mempoolWatcher()
	go tokenListWatcher()

	time.Sleep(5 * time.Second)
	blockProcessed[-1] = ProcessedBeaconBestState
	cacheOTAMap = make(map[string]string)
	for i := 0; i < Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
		blockProcessed[i] = ShardProcessedState[byte(i)]
		go checkMissingCrossShardTx(i, ShardProcessedState[byte(i)])
		Localnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
	Localnode.OnNewBlockFromParticularHeight(-1, int64(ProcessedBeaconBestState), true, processBeacon)

}

func ResetMongoAndReSync() error {
	dir := chainDataFolder + "/database"
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Println(err)
		return nil
	}

	for _, f := range files {
		fileName := f.Name()
		if fileName == "userdb" || fileName == "beacon" {
			continue
		}
		err := os.RemoveAll(dir + "/" + fileName)
		if err != nil {
			return err
		}
	}

	_, _, db, _ := mgm.DefaultConfigs()
	err = db.Drop(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func getCrossShardData(result map[string]string, txList []metadata.Transaction, shardID byte) error {
	for _, tx := range txList {
		var prvProof privacy.Proof
		txHash := tx.Hash().String()
		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			customTokenPrivacyTx, ok := tx.(transaction.TransactionToken)
			if !ok {
				return errors.New("Cannot cast transaction")
			}
			prvProof = customTokenPrivacyTx.GetTxBase().GetProof()
			txTokenData := customTokenPrivacyTx.GetTxTokenData()
			txTokenProof := txTokenData.TxNormal.GetProof()
			if txTokenProof != nil {
				for _, outCoin := range txTokenProof.GetOutputCoins() {
					coinShardID, err := outCoin.GetShardID()
					if err == nil && coinShardID == shardID {
						result[outCoin.GetCommitment().String()] = txHash
					}
				}
			}
		} else {
			prvProof = tx.GetProof()
		}
		if prvProof != nil {
			for _, outCoin := range prvProof.GetOutputCoins() {
				coinShardID, err := outCoin.GetShardID()
				if err == nil && coinShardID == shardID {
					result[outCoin.GetCommitment().String()] = txHash
				}
			}
		}
	}

	return nil
}

func getCrossShardDataPubkey(result map[string]string, txList []metadata.Transaction, shardID byte) error {
	for _, tx := range txList {
		var prvProof privacy.Proof
		txHash := tx.Hash().String()
		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			customTokenPrivacyTx, ok := tx.(transaction.TransactionToken)
			if !ok {
				return errors.New("Cannot cast transaction")
			}
			prvProof = customTokenPrivacyTx.GetTxBase().GetProof()
			txTokenData := customTokenPrivacyTx.GetTxTokenData()
			txTokenProof := txTokenData.TxNormal.GetProof()
			if txTokenProof != nil {
				for _, outCoin := range txTokenProof.GetOutputCoins() {
					coinShardID, err := outCoin.GetShardID()
					if err == nil && coinShardID == shardID {
						if outCoin.GetVersion() == 2 {
							result[base58.EncodeCheck(outCoin.GetPublicKey().ToBytesS())] = txHash
						}
					}
				}
			}
		} else {
			prvProof = tx.GetProof()
		}
		if prvProof != nil {
			for _, outCoin := range prvProof.GetOutputCoins() {
				coinShardID, err := outCoin.GetShardID()
				if err == nil && coinShardID == shardID {
					if outCoin.GetVersion() == 2 {
						result[base58.EncodeCheck(outCoin.GetPublicKey().ToBytesS())] = txHash
					}
				}
			}
		}
	}

	return nil
}

var cacheOTAMap map[string]string
var cacheLck sync.RWMutex

func checkMissingCrossShardTx(shardID int, currentHeight uint64) {
	statePrefix := fmt.Sprintf("fixCRSSshard-%v", shardID)
	v, err := Localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
	if err != nil {
		log.Println(err)
		currentHeight = currentHeight
	} else {
		currentHeight, err = strconv.ParseUint(string(v), 0, 64)
		if err != nil {
			panic(err)
		}
	}
	for {
		if currentHeight <= config.Param().CoinVersion2LowestHeight {
			break
		}
		coinList := getCrossShardTxs(shardID, currentHeight)
		fmt.Println("checkMissingCrossShardTx", shardID, currentHeight, len(coinList))
		for coinPubkey, txhash := range coinList {
			if shared.BurnCoinID == coinPubkey {
				continue
			}
			coins, err := database.DBGetCoinV2ByPubkey([]string{coinPubkey})
			if err != nil {
				log.Fatalln("DBGetCoinV2ByPubkey", err)
			}
			if len(coins) != 1 {
				panic(fmt.Sprint("len(coins) != 1 "+coinPubkey, " ", len(coins), " ", txhash))
			}
			for _, v := range coins {
				if v.OTASecret != "" {
					cacheLck.RLock()
					pubkey, ok := cacheOTAMap[v.OTASecret]
					cacheLck.RUnlock()
					if !ok {
						key, err := database.DBGetCoinV2PubkeyInfoByOTAsecret(v.OTASecret)
						if err != nil {
							log.Fatalln("DBGetCoinV2PubkeyInfo", err)
						}
						if key == nil {
							panic("otakey not found " + v.OTASecret)
						}
						cacheLck.Lock()
						cacheOTAMap[v.OTASecret] = key.Pubkey
						cacheLck.Unlock()
						pubkey = key.Pubkey
					}
					err = database.DBUpdateTxPubkeyReceiverAndTokenID([]string{txhash}, pubkey, v.RealTokenID, context.Background())
					if err != nil {
						log.Fatalln("DBUpdateTxPubkeyReceiverAndTokenID", err)
					}
				}
				err := database.DBUpdateCoinsTx(v.CoinPubkey, txhash)
				if err != nil {
					log.Fatalln("DBUpdateCoinsTx", err)
				}
			}
		}
		err := Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", currentHeight)), nil)
		if err != nil {
			panic(err)
		}
		currentHeight -= 1
	}

}

func getCrossShardTxs(shardID int, height uint64) map[string]string {
	result := make(map[string]string)
	blkInterface, err := Localnode.GetBlockchain().GetBlockByHeight(proto.BlkType_BlkShard, height, byte(shardID), byte(shardID))
	if err != nil {
		panic(err)
	}
	blk := blkInterface.(*types.ShardBlock)
	for sID, txlist := range blk.Body.CrossTransactions {
		for _, tx := range txlist {
			blkHash := &tx.BlockHash
			i := 0
		retryGetBlock2:
			if i == 50 {
				panic("OnNewShardBlock err " + blkHash.String())
			}
			blkInterface, err := Localnode.GetBlockchain().GetBlockByHash(proto.BlkType_BlkShard, blkHash, byte(sID), byte(sID))
			if err != nil {
				fmt.Println(err)
				i++
				time.Sleep(5 * time.Second)
				goto retryGetBlock2
			}
			crsblk := blkInterface.(*types.ShardBlock)
			err = getCrossShardDataPubkey(result, crsblk.Body.Transactions, byte(shardID))
			if err != nil {
				panic(err)
			}
		}
	}

	return result
}
