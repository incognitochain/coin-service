package chainsynker

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/wallet"
	jsoniter "github.com/json-iterator/go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

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
	"github.com/kamva/mgm/v3/operator"
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
	if cfg.FullnodeData == false {
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
	node := devframework.NewAppNode(chainDataFolder, netw, true, false, false, cfg.EnableChainLog)
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

	// time.Sleep(5 * time.Second)
	// go func() {
	// _, err = Localnode.GetUserDatabase().Get([]byte("checkMissingTxs"), nil)
	// if err != nil {
	// 	if err != leveldb.ErrNotFound {
	// 		panic(err)
	// 	} else {
	// 		shardsHash := Localnode.GetBlockchain().GetBeaconBestState().GetBestShardHash()
	// 		shardsHeight := Localnode.GetBlockchain().GetBeaconBestState().GetBestShardHeight()
	// 		var wg sync.WaitGroup
	// 		for sID, v := range shardsHash {
	// 			wg.Add(1)
	// 			go func(sid int, height uint64, hash common.Hash) {
	// 				defer wg.Done()
	// 				for {
	// 					hash, height, sid, err = checkMissingTxs(hash, height, sid)
	// 					if err != nil {
	// 						panic(err)
	// 					}
	// 					if height == 1 {
	// 						return
	// 					}
	// 				}
	// 			}(int(sID), shardsHeight[sID], v)
	// 		}
	// 		wg.Wait()
	// 		err = Localnode.GetUserDatabase().Put([]byte("checkMissingTxs"), []byte{1}, nil)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 	}
	// }
	// }()
	time.Sleep(5 * time.Second)
	blockProcessed[-1] = ProcessedBeaconBestState
	for i := 0; i < Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
		blockProcessed[i] = ShardProcessedState[byte(i)]
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

func getTokenID(assetTag string) string {
retry:
	if len(lastTokenIDMap) == 0 {
		time.Sleep(5 * time.Second)
		fmt.Println("ahaaaaaa could be stuck here 1")
		goto retry
	}
	lastTokenIDLock.RLock()
	tokenIDStr, ok := lastTokenIDMap[assetTag]
	if !ok {
		time.Sleep(5 * time.Second)
		fmt.Println("ahaaaaaa could be stuck here 2")
		lastTokenIDLock.RUnlock()
		goto retry
	}
	lastTokenIDLock.RUnlock()
	return tokenIDStr
}

func extractInstructionFromBeaconBlocks(shardPrevBlkHash []byte, currentBeaconHeight uint64) ([][]string, error) {
	var blk types.ShardBlock
	blkBytes, err := Localnode.GetUserDatabase().Get(shardPrevBlkHash, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		panic(err)
	}
	var insts [][]string
	for i := blk.Header.BeaconHeight + 1; i <= currentBeaconHeight; i++ {
		bblk, err := Localnode.GetBlockchain().GetBeaconBlockByHeight(i)
		if err != nil {
			return nil, err
		}
		insts = append(insts, bblk[0].Body.Instructions...)
	}
	return insts, nil
}
func checkMissingTxs(blockHash common.Hash, height uint64, chainID int) (common.Hash, uint64, int, error) {
	fmt.Println("checkMissingTxs", height, chainID)
	var blk types.ShardBlock
	blkBytes, err := Localnode.GetUserDatabase().Get(blockHash.Bytes(), nil)
	if err != nil {
		i := 0
	retry:
		if i >= 50 {
			fmt.Println(blockHash, height, chainID, err)
			panic("OnNewShardBlock err")
		}
		fmt.Println("err get block height", height, blockHash.String())
		blkBytes, err = Localnode.SyncSpecificShardBlockBytes(chainID, height, blockHash.String())
		if err != nil {
			fmt.Println(err)
			i++
			time.Sleep(5 * time.Second)
			goto retry
		}
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		panic(err)
	}
	blockHeight := blk.GetHeight()
	shardID := blk.GetShardID()

	txDataList := []shared.TxData{}
	for _, tx := range blk.Body.Transactions {
		pubkey := ""
		_ = pubkey
		txBytes, err := json.Marshal(tx)
		if err != nil {
			panic(err)
		}
		txHash := tx.Hash().String()
		tokenID := tx.GetTokenID().String()
		metaDataType := tx.GetMetadataType()
		outputLens := 0
		if metaDataType == metadata.PDECrossPoolTradeRequestMeta || metaDataType == metadata.PDETradeRequestMeta {
			realTokenID := ""
			txKeyImages := []string{}
			if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
				if tx.GetProof() == nil {
					panic(txHash)
					continue
				}
				realTokenID = common.PRVCoinID.String()
				ins := tx.GetProof().GetInputCoins()
				outs := tx.GetProof().GetOutputCoins()
				outputLens = len(outs)
				for _, coin := range ins {
					kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
					txKeyImages = append(txKeyImages, kmString)
				}
			}

			if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
				txToken := tx.(transaction.TransactionToken)
				txTokenData := txToken.GetTxTokenData()

				if txTokenData.TxNormal.GetProof() != nil {
					tokenIns := txTokenData.TxNormal.GetProof().GetInputCoins()
					tokenOuts := txTokenData.TxNormal.GetProof().GetOutputCoins()
					outputLens += len(tokenOuts)
					for _, coin := range tokenIns {
						kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
						txKeyImages = append(txKeyImages, kmString)
					}
				}
				if tx.GetTxFee() > 0 {
					ins := tx.GetProof().GetInputCoins()
					outs := tx.GetProof().GetOutputCoins()
					outputLens += len(outs)
					for _, coin := range ins {
						kmString := base58.EncodeCheck(coin.GetKeyImage().ToBytesS())
						txKeyImages = append(txKeyImages, kmString)
					}
				}
			}

			switch metaDataType {
			case metadata.PDECrossPoolTradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDECrossPoolTradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					log.Println(err)
				} else {
					pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
				}
			case metadata.PDETradeRequestMeta:
				meta := tx.GetMetadata().(*metadata.PDETradeRequest)
				payment := meta.TraderAddressStr
				wl, err := wallet.Base58CheckDeserialize(payment)
				if err != nil {
					log.Println(err)
				} else {
					pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk)
				}
			}

			mtd := ""
			if tx.GetMetadata() != nil {
				mtdBytes, err := json.Marshal(tx.GetMetadata())
				if err != nil {
					panic(err)
				}
				mtd = string(mtdBytes)
			}
			pubkeyReceivers := []string{}
			if tx.GetVersion() == 1 {
				realTokenID = tx.GetTokenID().String()
				receiverList, _ := tx.GetReceivers()
				for _, v := range receiverList {
					pubkeyReceivers = append(pubkeyReceivers, base58.EncodeCheck(v))
				}
				if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
					txToken := tx.(transaction.TransactionToken)
					receiverList, _ := txToken.GetTxTokenData().TxNormal.GetReceivers()
					for _, v := range receiverList {
						pubkeyReceivers = append(pubkeyReceivers, base58.EncodeCheck(v))
					}
				}
			}
			if tx.GetVersion() == 2 && outputLens > 0 {
				pubkeys, realTk, err := dbGetRealTokenIDByTxHash(txHash, shardID)
				if err != nil {
					panic(err)
					continue
				}
				if realTokenID == "" {
					realTokenID = realTk
				}
				pubkeyReceivers = append(pubkeyReceivers, pubkeys...)
			}
			///////////////

			txData := shared.NewTxData(tx.GetLockTime(), shardID, int(tx.GetVersion()), blockHeight, blockHash.String(), tokenID, txHash, tx.GetType(), string(txBytes), strconv.Itoa(metaDataType), mtd, txKeyImages, pubkeyReceivers, false)
			txData.RealTokenID = realTokenID
			if tx.GetVersion() == 2 {
				txData.PubKeyReceivers = append(txData.PubKeyReceivers, pubkey)
			}
			txDataList = append(txDataList, *txData)
		}

	}

	if len(txDataList) > 0 {
		err := database.DBSaveTXs(txDataList)
		if err != nil {
			panic(err)
		}
	}

	return blk.Header.PreviousBlockHash, blockHeight - 1, shardID, nil
}

func dbGetRealTokenIDByTxHash(txhash string, shardID int) ([]string, string, error) {
	limit := int64(1)
	pubkeys := []string{}
	realTokenID := ""
	list := []shared.CoinData{}
	if limit == 0 {
		limit = 10
	}
	filter := bson.M{"txhash": bson.M{operator.Eq: txhash}}
	err := mgm.Coll(&shared.CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		Sort:  bson.D{{"coinidx", 1}},
		Limit: &limit,
	})
	if err != nil {
		return nil, "", err
	}
	if len(list) == 0 {
		fmt.Println("txhash", txhash, shardID)
		panic("dbGetRealTokenIDByTxHash")
	}
	otakeys := make(map[string]struct{})
	for _, v := range list {
		if v.CoinPubkey == "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA" {
			continue
		} else {
			otakeys[v.OTASecret] = struct{}{}
			if v.RealTokenID != "" && v.RealTokenID != "0000000000000000000000000000000000000000000000000000000000000004" {
				realTokenID = v.RealTokenID
			}
		}
	}

	for otakey, _ := range otakeys {
		key := shared.KeyInfoDataV2{}
	retry:
		result := mgm.Coll(&shared.KeyInfoDataV2{}).FindOne(context.Background(), bson.M{"otakey": bson.M{operator.Eq: otakey}})
		if result.Err() == nil {
			err = result.Decode(&key)
			if err != nil {
				panic(err)
			}
			pubkeys = append(pubkeys, key.Pubkey)
		} else {
			if result.Err() != mongo.ErrNoDocuments {
				fmt.Println("KeyInfoDataV2", txhash)
				goto retry
			}
		}
	}

	return pubkeys, realTokenID, nil
}
