package chainsynker

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
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
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/kamva/mgm/v3"
	"github.com/syndtr/goleveldb/leveldb"
	"go.mongodb.org/mongo-driver/mongo"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var Localnode interface {
	GetUserDatabase() *leveldb.DB
	GetBlockchain() *blockchain.BlockChain
	OnNewBlockFromParticularHeight(chainID int, blkHeight int64, isFinalized bool, f func(bc *blockchain.BlockChain, h common.Hash, height uint64))
	GetShardState(shardID int) (uint64, *common.Hash)
}

var ShardProcessedState map[byte]uint64
var TransactionStateDB map[byte]*statedb.StateDB

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

	//store output-coin and keyimage on db
	keyImageList := []shared.KeyImageData{}
	outCoinList := []shared.CoinData{}
	beaconHeight := blk.Header.BeaconHeight
	coinV1PubkeyInfo := make(map[string]map[string]shared.CoinInfo)

	txDataList := []shared.TxData{}

	for _, txs := range blk.Body.CrossTransactions {
		for _, tx := range txs {
			for _, prvout := range tx.OutputCoin {
				publicKeyBytes := prvout.GetPublicKey().ToBytesS()
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
						if _, ok := coinV1PubkeyInfo[prvout.GetPublicKey().String()]; !ok {
							coinV1PubkeyInfo[prvout.GetPublicKey().String()] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, prvout.Bytes(), common.PRVCoinID.String(), prvout.GetPublicKey().String(), "", tx.Hash().String(), shardID, int(prvout.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
			for _, tkouts := range tx.TokenPrivacyData {
				for _, tkout := range tkouts.OutputCoin {
					publicKeyBytes := tkout.GetPublicKey().ToBytesS()
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
							if _, ok := coinV1PubkeyInfo[tkout.GetPublicKey().String()]; !ok {
								coinV1PubkeyInfo[tkout.GetPublicKey().String()] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr]; !ok {
								coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, tkout.Bytes(), tokenStr, tkout.GetPublicKey().String(), "", tx.Hash().String(), shardID, int(tkout.GetVersion()))
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
						if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
							coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]shared.CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = shared.CoinInfo{
								Start: coinIdx,
								Total: 1,
								End:   coinIdx,
							}
						} else {
							newCoinInfo := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]
							newCoinInfo.Total = newCoinInfo.Total + 1
							if coinIdx > newCoinInfo.End {
								newCoinInfo.End = coinIdx
							}
							if coinIdx < newCoinInfo.Start {
								newCoinInfo.Start = coinIdx
							}
							coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = newCoinInfo
						}
					}
					outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenID, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
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
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenStr, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
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
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]shared.CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = shared.CoinInfo{
									Start: coinIdx,
									Total: 1,
									End:   coinIdx,
								}
							} else {
								newCoinInfo := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]
								newCoinInfo.Total = newCoinInfo.Total + 1
								if coinIdx > newCoinInfo.End {
									newCoinInfo.End = coinIdx
								}
								if coinIdx < newCoinInfo.Start {
									newCoinInfo.Start = coinIdx
								}
								coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = newCoinInfo
							}
						}
						outCoin := shared.NewCoinData(beaconHeight, coinIdx, coin.Bytes(), common.PRVCoinID.String(), coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
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
		txData := shared.NewTxData(tx.GetLockTime(), shardID, int(tx.GetVersion()), blockHash, blockHeight, tokenID, tx.Hash().String(), tx.GetType(), tx.String(), txKeyImages, pubkeyReceivers)
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
	fullnodeAddress string
)

func InitChainSynker(cfg shared.Config) {
	chainNetwork = cfg.ChainNetwork
	highwayAddress = cfg.Highway
	chainDataFolder = cfg.ChainDataFolder
	fullnodeAddress = cfg.FullnodeAddress

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
	node := devframework.NewAppNode(chainDataFolder, netw, true, false, false)
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
	for i := 0; i < Localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		Localnode.OnNewBlockFromParticularHeight(i, int64(ShardProcessedState[byte(i)]), true, OnNewShardBlock)
	}
	go mempoolWatcher()
	go tokenListWatcher()
}
func mempoolWatcher() {
	interval := time.NewTicker(5 * time.Second)
	rpcclient := devframework.NewRPCClient(fullnodeAddress)
	for {
		<-interval.C
		mempoolTxs, err := rpcclient.API_GetRawMempool()
		if err != nil {
			log.Println(err)
			continue
		}
		var pendingTxs []shared.CoinPendingData
		for _, txHash := range mempoolTxs.TxHashes {
			txDetail, err := shared.GetTxByHash(fullnodeAddress, txHash)
			if err != nil {
				log.Println(err)
				continue
			}
			var sn []string
			for _, c := range txDetail.Result.ProofDetail.InputCoins {
				sn = append(sn, c.CoinDetails.SerialNumber)
			}
			pendingTxs = append(pendingTxs, *shared.NewCoinPendingData(sn, txHash))
		}
		err = database.DBSavePendingTx(pendingTxs)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}
func tokenListWatcher() {
	interval := time.NewTicker(10 * time.Second)
	rpcclient := devframework.NewRPCClient(fullnodeAddress)
	for {
		<-interval.C
		tokenList, err := rpcclient.API_ListPrivacyCustomToken()
		if err != nil {
			log.Println(err)
			continue
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
	}
}

func syncUnfinalizedShard() {

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
