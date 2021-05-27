package chainsynker

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"sync"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
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
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wire"
	"github.com/kamva/mgm/v3"
	"github.com/syndtr/goleveldb/leveldb"
	"go.mongodb.org/mongo-driver/mongo"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var Localnode interface {
	OnReceive(msgType int, f func(msg interface{}))
	GetUserDatabase() *leveldb.DB
	GetBlockchain() *blockchain.BlockChain
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
	GetShardState(shardID int) (uint64, *common.Hash)
}

var ShardProcessedState map[byte]uint64
var TransactionStateDB map[byte]*statedb.StateDB
var lastTokenIDMap map[string]string
var lastTokenIDLock sync.RWMutex

func OnNewShardBlock(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	var blk types.ShardBlock
	blkBytes, err := Localnode.GetUserDatabase().Get(h.Bytes(), nil)
	if err != nil {
		log.Println(err)
		return
	}
	if err := json.Unmarshal(blkBytes, &blk); err != nil {
		log.Println(err)
		return
	}
	blockHash := blk.Hash().String()
	blockHeight := fmt.Sprintf("%v", blk.GetHeight())
	shardID := blk.GetShardID()

	if len(blk.Body.Transactions) > 0 {
		err = bc.CreateAndSaveTxViewPointFromBlock(&blk, TransactionStateDB[byte(shardID)])
		if err != nil {
			panic(err)
		}
	}
	// Store Incomming Cross Shard
	if len(blk.Body.CrossTransactions) > 0 {
		if err := bc.CreateAndSaveCrossTransactionViewPointFromBlock(&blk, TransactionStateDB[byte(shardID)]); err != nil {
			panic(err)
		}
	}
	transactionRootHash, err := TransactionStateDB[byte(shardID)].Commit(true)
	if err != nil {
		panic(err)
	}
	err = TransactionStateDB[byte(shardID)].Database().TrieDB().Commit(transactionRootHash, false)
	if err != nil {
		panic(err)
	}
	bc.GetBestStateShard(byte(blk.GetShardID())).TransactionStateDBRootHash = transactionRootHash

	TransactionStateDB[byte(shardID)].ClearObjects()

	batchData := bc.GetShardChainDatabase(blk.Header.ShardID).NewBatch()
	err = bc.BackupShardViews(batchData, blk.Header.ShardID)
	if err != nil {
		panic("Backup shard view error")
	}

	if err := batchData.Write(); err != nil {
		panic(err)
	}

	crossShardCoinMap := make(map[string]string)
	for _, txlist := range blk.Body.CrossTransactions {
		for _, tx := range txlist {
			var crsblk types.ShardBlock
		retryGetBlock:
			blkBytes, err := Localnode.GetUserDatabase().Get(tx.BlockHash.Bytes(), nil)
			if err != nil {
				log.Println(err)
				time.Sleep(5 * time.Second)
				goto retryGetBlock
			}
			if err := json.Unmarshal(blkBytes, &crsblk); err != nil {
				log.Println(err)
				return
			}
			getCrossShardData(crossShardCoinMap, crsblk.Body.Transactions, byte(shardID))
		}
	}

	//store output-coin and keyimage on db
	keyImageList := []shared.KeyImageData{}
	outCoinList := []shared.CoinData{}
	beaconHeight := blk.Header.BeaconHeight
	coinV1PubkeyInfo := make(map[string]map[string]shared.CoinInfo)

	txDataList := []shared.TxData{}
	tradeRespondList := []shared.TradeData{}
	for _, txs := range blk.Body.CrossTransactions {
		for _, tx := range txs {
			for _, prvout := range tx.OutputCoin {
				publicKeyBytes := prvout.GetPublicKey().ToBytesS()
				publicKeyStr := base58.EncodeCheck(publicKeyBytes)
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					if prvout.GetVersion() == 2 {
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {
							log.Println("len(outs))", len(tx.OutputCoin), base58.Base58Check{}.Encode(publicKeyBytes, 0))
							panic(err)
						}
						coinIdx = idxBig.Uint64()
					} else {
						idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, prvout.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
						if err != nil {
							panic(err)
						}
						coinIdx = idxBig.Uint64()
						if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
							coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, prvout.Bytes(), common.PRVCoinID.String(), publicKeyStr, "", crossShardCoinMap[prvout.GetCommitment().String()], shardID, int(prvout.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
			for _, tkouts := range tx.TokenPrivacyData {
				for _, tkout := range tkouts.OutputCoin {
					publicKeyBytes := tkout.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						tokenStr := tkouts.PropertyID.String()
						if tkout.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], tkouts.PropertyID, publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							tokenStr = common.ConfidentialAssetID.String()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], tkouts.PropertyID, tkout.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][tokenStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, tkout.Bytes(), tokenStr, publicKeyStr, "", crossShardCoinMap[tkout.GetCommitment().String()], shardID, int(tkout.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
		}
	}

	for _, tx := range blk.Body.Transactions {
		isCoinV2Output := false
		txHash := tx.Hash().String()
		tokenID := tx.GetTokenID().String()
		txKeyImages := []string{}
		if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
			if tx.GetProof() == nil {
				continue
			}
			ins := tx.GetProof().GetInputCoins()
			outs := tx.GetProof().GetOutputCoins()

			for _, coin := range ins {
				kmString := base64.StdEncoding.EncodeToString(coin.GetKeyImage().ToBytesS())
				txKeyImages = append(txKeyImages, kmString)
				kmData := shared.NewKeyImageData(tokenID, txHash, kmString, beaconHeight, shardID)
				keyImageList = append(keyImageList, *kmData)
			}
			for _, coin := range outs {
				publicKeyBytes := coin.GetPublicKey().ToBytesS()
				publicKeyStr := base58.EncodeCheck(publicKeyBytes)
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					if coin.GetVersion() == 2 {
						isCoinV2Output = true
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {
							log.Println("len(outs))", len(outs), base58.Base58Check{}.Encode(publicKeyBytes, 0))
							panic(err)
						}
						coinIdx = idxBig.Uint64()
					} else {
						idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
						if err != nil {
							panic(err)
						}
						coinIdx = idxBig.Uint64()
						if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
							coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenID, publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
		}
		if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
			txToken := tx.(transaction.TransactionToken)
			txTokenData := txToken.GetTxTokenData()
			if txTokenData.TxNormal.GetProof() != nil {
				tokenIns := txTokenData.TxNormal.GetProof().GetInputCoins()
				tokenOuts := txTokenData.TxNormal.GetProof().GetOutputCoins()
				for _, coin := range tokenIns {
					kmString := base64.StdEncoding.EncodeToString(coin.GetKeyImage().ToBytesS())
					txKeyImages = append(txKeyImages, kmString)
					kmData := shared.NewKeyImageData(common.ConfidentialAssetID.String(), txHash, kmString, beaconHeight, shardID)
					keyImageList = append(keyImageList, *kmData)
				}
				for _, coin := range tokenOuts {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						tokenStr := txToken.GetTokenID().String()
						if coin.GetVersion() == 2 {
							isCoinV2Output = true
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], *tx.GetTokenID(), publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							tokenStr = common.ConfidentialAssetID.String()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], *txToken.GetTokenID(), coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][tokenStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenStr, publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
			if tx.GetTxFee() > 0 {
				ins := tx.GetProof().GetInputCoins()
				outs := tx.GetProof().GetOutputCoins()
				for _, coin := range ins {
					kmString := base64.StdEncoding.EncodeToString(coin.GetKeyImage().ToBytesS())
					txKeyImages = append(txKeyImages, kmString)
					kmData := shared.NewKeyImageData(common.PRVCoinID.String(), txHash, kmString, beaconHeight, shardID)
					keyImageList = append(keyImageList, *kmData)
				}
				for _, coin := range outs {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyStr := base58.EncodeCheck(publicKeyBytes)
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						if coin.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(blk.GetShardID()))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[publicKeyStr]; !ok {
								coinV1PubkeyInfo[publicKeyStr] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]; !ok {
								coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[publicKeyStr][common.PRVCoinID.String()] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), common.PRVCoinID.String(), publicKeyStr, "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
		}

		pubkeyReceivers := []string{}
		if !isCoinV2Output {
			receiverList, _ := tx.GetReceivers()
			for _, v := range receiverList {
				pubkeyReceivers = append(pubkeyReceivers, base58.EncodeCheck(v))
			}
		}
		txBytes, err := json.Marshal(tx)
		if err != nil {
			panic(err)
		}
		metaDataType := tx.GetMetadataType()

		if metaDataType == metadata.PDECrossPoolTradeResponseMeta || metaDataType == metadata.PDETradeResponseMeta {
			requestTx := ""
			status := ""
			switch metaDataType {
			case metadata.PDECrossPoolTradeResponseMeta:
				requestTx = tx.GetMetadata().(*metadata.PDECrossPoolTradeResponse).RequestedTxID.String()
				status = tx.GetMetadata().(*metadata.PDECrossPoolTradeResponse).TradeStatus
			case metadata.PDETradeResponseMeta:
				requestTx = tx.GetMetadata().(*metadata.PDETradeResponse).RequestedTxID.String()
				status = tx.GetMetadata().(*metadata.PDETradeResponse).TradeStatus
			}
			outs := []coin.Coin{}
			tokenIDStr := tx.GetTokenID().String()
			if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
				txToken := tx.(transaction.TransactionToken)
				outs = txToken.GetTxTokenData().TxNormal.GetProof().GetOutputCoins()
				if isCoinV2Output {
				retry:
					if len(lastTokenIDMap) == 0 {
						time.Sleep(10 * time.Second)
						goto retry
					}
					lastTokenIDLock.RLock()
					var ok bool
					tokenIDStr, ok = lastTokenIDMap[outs[0].GetAssetTag().String()]
					if !ok {
						time.Sleep(10 * time.Second)
						lastTokenIDLock.RUnlock()
						goto retry
					}
					lastTokenIDLock.RUnlock()
				}
			} else {
				outs = tx.GetProof().GetOutputCoins()
			}
			trade := shared.NewTradeData(requestTx, tx.Hash().String(), status, tokenIDStr, outs[0].GetValue())
			tradeRespondList = append(tradeRespondList, *trade)
		}
		mtd := ""
		if tx.GetMetadata() != nil {
			mtdBytes, err := json.Marshal(tx.GetMetadata())
			if err != nil {
				panic(err)
			}
			mtd = string(mtdBytes)
		}
		txData := shared.NewTxData(tx.GetLockTime(), shardID, int(tx.GetVersion()), blockHash, blockHeight, tokenID, tx.Hash().String(), tx.GetType(), string(txBytes), strconv.Itoa(metaDataType), mtd, txKeyImages, pubkeyReceivers)
		txDataList = append(txDataList, *txData)
	}
	alreadyWriteToBD := false

	if len(txDataList) > 0 {
		err = database.DBSaveTXs(txDataList)
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				log.Println(err)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			}
		}
	}
	if len(outCoinList) > 0 {
		err = database.DBSaveCoins(outCoinList)
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				log.Println(err)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			} else {
				alreadyWriteToBD = true
			}
		}
	}
	if len(keyImageList) > 0 {
		err = database.DBSaveUsedKeyimage(keyImageList)
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				log.Println(err)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			}
		}
	}
	if len(tradeRespondList) > 0 {
		err = database.DBSaveTxTrade(tradeRespondList)
		if err != nil {
			panic(err)
		}
	}
	if len(coinV1PubkeyInfo) > 0 && !alreadyWriteToBD {
		err = database.DBUpdateCoinV1PubkeyInfo(coinV1PubkeyInfo)
		if err != nil {
			panic(err)
		}
	}
	statePrefix := fmt.Sprintf("coin-processed-%v", blk.Header.ShardID)
	err = Localnode.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", blk.Header.Height)), nil)
	if err != nil {
		panic(err)
	}
}

var (
	chainNetwork    string
	highwayAddress  string
	chainDataFolder string
	// fullnodeAddress string
)

func InitChainSynker(cfg shared.Config) {
	lastTokenIDMap = make(map[string]string)
	chainNetwork = cfg.ChainNetwork
	highwayAddress = cfg.Highway
	chainDataFolder = cfg.ChainDataFolder
	// fullnodeAddress = cfg.FullnodeAddress

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

	var netw devframework.NetworkParam
	switch chainNetwork {
	case "testnet2":
		netw = devframework.TestNet2Param
	case "testnet":
		netw = devframework.TestNetParam
	case "mainnet":
		netw = devframework.MainNetParam
	default:
		panic("unknown network")
	}
	netw.HighwayAddress = highwayAddress
	node := devframework.NewAppNode(chainDataFolder, netw, true, false, false, cfg.EnableChainLog)
	Localnode = node
	log.Println("initiating chain-synker...")
	if shared.RESET_FLAG {
		for i := 0; i < Localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
			statePrefix := fmt.Sprintf("coin-processed-%v", i)
			err := Localnode.GetUserDatabase().Delete([]byte(statePrefix), nil)
			if err != nil {
				panic(err)
			}
		}
		log.Println("=========================")
		log.Println("RESET SUCCESS")
		log.Println("=========================")
	}
	ShardProcessedState = make(map[byte]uint64)
	TransactionStateDB = make(map[byte]*statedb.StateDB)

	for i := 0; i < Localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		statePrefix := fmt.Sprintf("coin-processed-%v", i)
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
		TransactionStateDB[byte(i)] = Localnode.GetBlockchain().GetBestStateShard(byte(i)).GetCopiedTransactionStateDB()
	}
	go mempoolWatcher()
	go tokenListWatcher()
	time.Sleep(2 * time.Second)
	for i := 0; i < Localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		Localnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
	Localnode.OnNewBlockFromParticularHeight(-1, int64(Localnode.GetBlockchain().BeaconChain.GetFinalViewHeight()), true, updatePDEState)

}

func updatePDEState(bc *blockchain.BlockChain, h common.Hash, height uint64) {
	beaconBestState, _ := Localnode.GetBlockchain().GetBeaconViewStateDataFromBlockHash(h, false)
	beaconFeatureStateRootHash := beaconBestState.FeatureStateDBRootHash

	beaconFeatureStateDB, err := statedb.NewWithPrefixTrie(beaconFeatureStateRootHash, statedb.NewDatabaseAccessWarper(Localnode.GetBlockchain().GetBeaconChainDatabase()))
	if err != nil {
		log.Println(err)
	}
	pdeState, err := blockchain.InitCurrentPDEStateFromDB(beaconFeatureStateDB, nil, beaconBestState.BeaconHeight)
	if err != nil {
		log.Println(err)
	}
	pdeStateJSON := jsonresult.CurrentPDEState{
		BeaconTimeStamp:         beaconBestState.BestBlock.Header.Timestamp,
		PDEPoolPairs:            pdeState.PDEPoolPairs,
		PDEShares:               pdeState.PDEShares,
		WaitingPDEContributions: pdeState.WaitingPDEContributions,
		PDETradingFees:          pdeState.PDETradingFees,
	}
	pdeStr, err := json.MarshalToString(pdeStateJSON)
	if err != nil {
		log.Println(err)
	}
	err = database.DBSavePDEState(pdeStr)
	if err != nil {
		log.Println(err)
	}
}

func mempoolWatcher() {
	Localnode.OnReceive(devframework.MSG_TX, func(msg interface{}) {
		msgData := msg.(*wire.MessageTx)
		if msgData.Transaction.GetProof() != nil {
			var sn []string
			var pendingTxs []shared.CoinPendingData
			for _, c := range msgData.Transaction.GetProof().GetInputCoins() {
				sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
				shardID := common.GetShardIDFromLastByte(msgData.Transaction.GetSenderAddrLastByte())
				pendingTxs = append(pendingTxs, *shared.NewCoinPendingData(sn, int(shardID), msgData.Transaction.Hash().String()))
			}
			err := database.DBSavePendingTx(pendingTxs)
			if err != nil {
				log.Println(err)
			}
		}
	})
	Localnode.OnReceive(devframework.MSG_TX_PRIVACYTOKEN, func(msg interface{}) {
		msgData := msg.(*wire.MessageTxPrivacyToken)
		if msgData.Transaction.GetProof() != nil {
			var sn []string
			var pendingTxs []shared.CoinPendingData
			for _, c := range msgData.Transaction.GetProof().GetInputCoins() {
				sn = append(sn, base64.StdEncoding.EncodeToString(c.GetKeyImage().ToBytesS()))
				shardID := common.GetShardIDFromLastByte(msgData.Transaction.GetSenderAddrLastByte())
				pendingTxs = append(pendingTxs, *shared.NewCoinPendingData(sn, int(shardID), msgData.Transaction.Hash().String()))
			}
			err := database.DBSavePendingTx(pendingTxs)
			if err != nil {
				log.Println(err)
			}
		}
	})
	interval := time.NewTicker(10 * time.Second)
	for {
		<-interval.C
		txList, err := database.DBGetPendingTxs()
		if err != nil {
			log.Println(err)
			continue
		}
		txsToRemove := []string{}
		for shardID, txHashes := range txList {
			exist, err := database.DBCheckTxsExist(txHashes, shardID)
			if err != nil {
				log.Println(err)
				continue
			}
			for idx, v := range exist {
				if v {
					txsToRemove = append(txsToRemove, txHashes[idx])
				}
			}
		}
		database.DBDeletePendingTxs(txsToRemove)
	}
}
func tokenListWatcher() {
	interval := time.NewTicker(10 * time.Second)
	for {
		<-interval.C
		shardStateDB := make(map[byte]*statedb.StateDB)
		for i := 0; i < Localnode.GetBlockchain().GetBeaconBestState().ActiveShards; i++ {
			shardID := byte(i)
			shardStateDB[shardID] = TransactionStateDB[shardID].Copy()
		}

		tokenStates := make(map[common.Hash]*statedb.TokenState)
		for i := 0; i < Localnode.GetBlockchain().GetBeaconBestState().ActiveShards; i++ {
			shardID := byte(i)
			m := statedb.ListPrivacyToken(shardStateDB[shardID])
			for newK, newV := range m {
				if v, ok := tokenStates[newK]; !ok {
					tokenStates[newK] = newV
				} else {
					if v.PropertyName() == "" && newV.PropertyName() != "" {
						v.SetPropertyName(newV.PropertyName())
					}
					if v.PropertySymbol() == "" && newV.PropertySymbol() != "" {
						v.SetPropertySymbol(newV.PropertySymbol())
					}
					v.AddTxs(newV.Txs())
				}
			}
		}

		tokenList := jsonresult.ListCustomToken{ListCustomToken: []jsonresult.CustomToken{}}
		for _, tokenState := range tokenStates {
			item := jsonresult.NewPrivacyToken(tokenState)
			tokenList.ListCustomToken = append(tokenList.ListCustomToken, *item)
		}

		_, allBridgeTokens, err := Localnode.GetBlockchain().GetAllBridgeTokens()
		if err != nil {
			log.Println(err)
			continue
		}

		for _, bridgeToken := range allBridgeTokens {
			if _, ok := tokenStates[*bridgeToken.TokenID]; ok {
				continue
			}
			item := jsonresult.CustomToken{
				ID:            bridgeToken.TokenID.String(),
				IsPrivacy:     true,
				IsBridgeToken: true,
			}
			if item.Name == "" {
				for i := 0; i < Localnode.GetBlockchain().GetBeaconBestState().ActiveShards; i++ {
					shardID := byte(i)
					tokenState, has, err := statedb.GetPrivacyTokenState(shardStateDB[shardID], *bridgeToken.TokenID)
					if err != nil {
						log.Println(err)
					}
					if has {
						item.Name = tokenState.PropertyName()
						item.Symbol = tokenState.PropertySymbol()
						break
					}
				}
			}
			tokenList.ListCustomToken = append(tokenList.ListCustomToken, item)
		}

		for index, _ := range tokenList.ListCustomToken {
			tokenList.ListCustomToken[index].ListTxs = []string{}
			tokenList.ListCustomToken[index].Image = common.Render([]byte(tokenList.ListCustomToken[index].ID))
			for _, bridgeToken := range allBridgeTokens {
				if tokenList.ListCustomToken[index].ID == bridgeToken.TokenID.String() {
					tokenList.ListCustomToken[index].Amount = bridgeToken.Amount
					tokenList.ListCustomToken[index].IsBridgeToken = true
					break
				}
			}
		}

		var tokenInfoList []shared.TokenInfoData
		for _, token := range tokenList.ListCustomToken {
			tokenInfo := shared.NewTokenInfoData(token.ID, token.Name, token.Symbol, token.Image, token.IsPrivacy, token.Amount)
			tokenInfoList = append(tokenInfoList, *tokenInfo)
		}
		err = database.DBSaveTokenInfo(tokenInfoList)
		if err != nil {
			log.Println(err)
			continue
		}
		lastTokenIDLock.Lock()

		if len(lastTokenIDMap) < len(tokenInfoList) {
			for _, tokenInfo := range tokenInfoList {
				tokenID, err := new(common.Hash).NewHashFromStr(tokenInfo.TokenID)
				if err != nil {
					log.Println(err)
					continue
				}
				recomputedAssetTag := operation.HashToPoint(tokenID[:])
				lastTokenIDMap[recomputedAssetTag.String()] = tokenInfo.TokenID
			}
		}

		lastTokenIDLock.Unlock()
	}
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
