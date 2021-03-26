package main

import (
	"bytes"

	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	devframework "github.com/0xkumi/incognito-dev-framework"
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
	GetShardState(shardID int) (uint64, *common.Hash)
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
	// Store Incomming Cross Shard
	if len(blk.Body.CrossTransactions) > 0 {
		if err := bc.CreateAndSaveCrossTransactionViewPointFromBlock(&blk, transactionStateDB); err != nil {
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
	// outcoinsIdx := make(map[string]uint64)
	coinV1PubkeyInfo := make(map[string]map[string]CoinInfo)

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
							fmt.Println("len(outs))", len(tx.OutputCoin), base58.Base58Check{}.Encode(publicKeyBytes, 0))
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
							coinV1PubkeyInfo[prvout.GetPublicKey().String()] = make(map[string]CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[prvout.GetPublicKey().String()][common.PRVCoinID.String()] = CoinInfo{
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
					outCoin := NewCoinData(beaconHeight, coinIdx, prvout.Bytes(), common.PRVCoinID.String(), prvout.GetPublicKey().String(), "", tx.Hash().String(), shardID, int(prvout.GetVersion()))
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
								coinV1PubkeyInfo[tkout.GetPublicKey().String()] = make(map[string]CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr]; !ok {
								coinV1PubkeyInfo[tkout.GetPublicKey().String()][tokenStr] = CoinInfo{
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
						outCoin := NewCoinData(beaconHeight, coinIdx, tkout.Bytes(), tokenStr, tkout.GetPublicKey().String(), "", tx.Hash().String(), shardID, int(tkout.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
			}
		}
	}

	for _, tx := range blk.Body.Transactions {
		txHash := tx.Hash().String()
		tokenID := tx.GetTokenID().String()
		if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
			fmt.Println("\n====================================================")
			fmt.Println(tokenID, txHash, tx.IsPrivacy(), tx.GetProof(), tx.GetVersion(), tx.GetMetadataType())
			if tx.GetProof() == nil {
				continue
			}
			ins := tx.GetProof().GetInputCoins()
			outs := tx.GetProof().GetOutputCoins()

			for _, coin := range ins {
				km := NewKeyImageData(tokenID, txHash, base58.Base58Check{}.Encode(coin.GetKeyImage().ToBytesS(), 0), beaconHeight, shardID)
				keyImageList = append(keyImageList, *km)
			}
			for _, coin := range outs {
				publicKeyBytes := coin.GetPublicKey().ToBytesS()
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					if coin.GetVersion() == 2 {
						idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(blk.GetShardID())], common.PRVCoinID, publicKeyBytes)
						if err != nil {
							fmt.Println("len(outs))", len(outs), base58.Base58Check{}.Encode(publicKeyBytes, 0))
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
							coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
							coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = CoinInfo{
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
					outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenID, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
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
				km := NewKeyImageData(common.ConfidentialAssetID.String(), txHash, base58.Base58Check{}.Encode(coin.GetKeyImage().ToBytesS(), 0), beaconHeight, shardID)
				keyImageList = append(keyImageList, *km)
			}
			for _, coin := range tokenOuts {
				publicKeyBytes := coin.GetPublicKey().ToBytesS()
				publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
				if publicKeyShardID == byte(shardID) {
					coinIdx := uint64(0)
					tokenStr := txToken.GetTokenID().String()
					if coin.GetVersion() == 2 {
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
							coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
						}
						if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr]; !ok {
							coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr] = CoinInfo{
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
					outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenStr, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
					outCoinList = append(outCoinList, *outCoin)
				}
			}
			fmt.Println(common.ConfidentialAssetID.String(), txHash, len(tokenIns), len(tokenOuts))
			fmt.Println("====================================================\n")
			if tx.GetTxFee() > 0 {
				ins := tx.GetProof().GetInputCoins()
				outs := tx.GetProof().GetOutputCoins()
				for _, coin := range ins {

					km := NewKeyImageData(common.PRVCoinID.String(), txHash, base58.Base58Check{}.Encode(coin.GetKeyImage().ToBytesS(), 0), beaconHeight, shardID)
					keyImageList = append(keyImageList, *km)
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
								coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = CoinInfo{
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
						outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), common.PRVCoinID.String(), coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
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
	if len(coinV1PubkeyInfo) > 0 {
		err = DBUpdateCoinV1PubkeyInfo(coinV1PubkeyInfo)
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

func initChainSynker() {
	err := DBCreateCoinV1Index()
	if err != nil {
		panic(err)
	}
	err = DBCreateKeyimageIndex()
	if err != nil {
		panic(err)
	}
	// devframework.TestNetParam.HighwayAddress = "139.162.55.124:9330"
	node := devframework.NewAppNode(serviceCfg.ChainDataFolder, devframework.TestNet2Param, true, false)
	localnode = node
	log.Println("initiating chain-synker...")
	ShardProcessedState = make(map[byte]uint64)
	TransactionStateDB = make(map[byte]*statedb.StateDB)
	//load ShardProcessedState
	p, err := localnode.GetUserDatabase().Get([]byte("genesis-processed"), nil)
	if err != nil {
		log.Println(err)
	}
	if p == nil {
		err := processGenesisBlocks()
		if err != nil {
			panic(err)
		}
		localnode.GetUserDatabase().Put([]byte("genesis-processed"), []byte{1}, nil)
		if err != nil {
			panic(err)
		}
	}
	for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		statePrefix := fmt.Sprintf("coin-processed-%v", i)
		v, err := localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
		if err != nil {
			log.Println(err)
		}
		if v != nil {
			height, err := strconv.ParseUint(string(v), 0, 64)
			if err != nil {
				log.Println(err)
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
	go mempoolWatcher(serviceCfg.FullnodeAddress)
}
func mempoolWatcher(fullnode string) {
	interval := time.NewTicker(5 * time.Second)
	rpcclient := devframework.NewRPCClient(fullnode)

	for {
		<-interval.C
		mempoolTxs, err := rpcclient.API_GetRawMempool()
		if err != nil {
			log.Println(err)
			continue
		}
		fmt.Println("mempoolTxs", mempoolTxs)
		var pendingTxs []CoinPendingData
		for _, txHash := range mempoolTxs.TxHashes {
			txDetail, err := getTxByHash(fullnode, txHash)
			if err != nil {
				log.Println(err)
				continue
			}
			var sn []string
			for _, c := range txDetail.Result.ProofDetail.InputCoins {
				sn = append(sn, c.CoinDetails.SerialNumber)
			}
			pendingTxs = append(pendingTxs, *NewCoinPendingData(sn, txHash))
		}
		fmt.Println("pendingTxs", pendingTxs)
		err = DBSavePendingTx(pendingTxs)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func getTxByHash(fullnode, txHash string) (*TxDetail, error) {
	query := fmt.Sprintf(`{
		"jsonrpc":"1.0",
		"method":"gettransactionbyhash",
		"params":["%s"],
		"id":1
	}`, txHash)
	var jsonStr = []byte(query)
	req, _ := http.NewRequest("POST", fullnode, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	autoTx, txError := ParseAutoTxHashFromBytes(body)
	if txError != nil {
		return nil, err
	}
	return autoTx, nil
}

func ParseAutoTxHashFromBytes(b []byte) (*TxDetail, error) {
	data := new(TxDetail)
	err := json.Unmarshal(b, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type TxDetail struct {
	ID     int `json:"Id"`
	Result struct {
		// BlockHash   string `json:"BlockHash"`
		// BlockHeight int    `json:"BlockHeight"`
		// TxSize      int    `json:"TxSize"`
		// Index       int    `json:"Index"`
		// ShardID     int    `json:"ShardID"`
		Hash string `json:"Hash"`
		// Version     int    `json:"Version"`
		// Type        string `json:"Type"`
		// LockTime    string `json:"LockTime"`
		// Fee         int    `json:"Fee"`
		// Image       string `json:"Image"`
		// IsPrivacy   bool   `json:"IsPrivacy"`
		// Proof       string `json:"Proof"`
		ProofDetail struct {
			InputCoins []struct {
				CoinDetails struct {
					PublicKey      string `json:"PublicKey"`
					CoinCommitment string `json:"CoinCommitment"`
					SNDerivator    struct {
					} `json:"SNDerivator"`
					SerialNumber string `json:"SerialNumber"`
					Randomness   struct {
					} `json:"Randomness"`
					Value int    `json:"Value"`
					Info  string `json:"Info"`
				} `json:"CoinDetails"`
				CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
			} `json:"InputCoins"`
			OutputCoins []struct {
				CoinDetails struct {
					PublicKey      string `json:"PublicKey"`
					CoinCommitment string `json:"CoinCommitment"`
					SNDerivator    struct {
					} `json:"SNDerivator"`
					SerialNumber string `json:"SerialNumber"`
					Randomness   struct {
					} `json:"Randomness"`
					Value int    `json:"Value"`
					Info  string `json:"Info"`
				} `json:"CoinDetails"`
				CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
			} `json:"OutputCoins"`
		} `json:"ProofDetail"`
		InputCoinPubKey string `json:"InputCoinPubKey"`
		// SigPubKey                     string `json:"SigPubKey"`
		// Sig                           string `json:"Sig"`
		// Metadata                      string `json:"Metadata"`
		// CustomTokenData               string `json:"CustomTokenData"`
		// PrivacyCustomTokenID          string `json:"PrivacyCustomTokenID"`
		// PrivacyCustomTokenName        string `json:"PrivacyCustomTokenName"`
		// PrivacyCustomTokenSymbol      string `json:"PrivacyCustomTokenSymbol"`
		// PrivacyCustomTokenData        string `json:"PrivacyCustomTokenData"`
		// PrivacyCustomTokenProofDetail struct {
		// 	InputCoins  interface{} `json:"InputCoins"`
		// 	OutputCoins interface{} `json:"OutputCoins"`
		// } `json:"PrivacyCustomTokenProofDetail"`
		// PrivacyCustomTokenIsPrivacy bool   `json:"PrivacyCustomTokenIsPrivacy"`
		// PrivacyCustomTokenFee       int    `json:"PrivacyCustomTokenFee"`
		// IsInMempool                 bool   `json:"IsInMempool"`
		// IsInBlock                   bool   `json:"IsInBlock"`
		// Info                        string `json:"Info"`
	} `json:"Result"`
	Error   interface{} `json:"Error"`
	Params  []string    `json:"Params"`
	Method  string      `json:"Method"`
	Jsonrpc string      `json:"Jsonrpc"`
}

func processGenesisBlocks() error {
	for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
		transactionStateDB := localnode.GetBlockchain().GetBestStateShard(byte(i)).GetCopiedTransactionStateDB()
		genesisBlock := localnode.GetBlockchain().ShardChain[i].Blockchain.GetChainParams().GenesisShardBlock
		outCoinList := []CoinData{}
		beaconHeight := genesisBlock.Header.BeaconHeight
		// outcoinsIdx := make(map[string]uint64)
		coinV1PubkeyInfo := make(map[string]map[string]CoinInfo)
		shardID := i
		for _, tx := range genesisBlock.Body.Transactions {
			txHash := tx.Hash().String()
			tokenID := tx.GetTokenID().String()
			if tx.GetType() == common.TxNormalType || tx.GetType() == common.TxConversionType || tx.GetType() == common.TxRewardType || tx.GetType() == common.TxReturnStakingType {
				fmt.Println("\n====================================================")
				fmt.Println(tokenID, txHash, tx.IsPrivacy(), tx.GetProof(), tx.GetVersion(), tx.GetMetadataType())
				if tx.GetProof() == nil {
					continue
				}
				outs := tx.GetProof().GetOutputCoins()

				for _, coin := range outs {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						if coin.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(shardID)], common.PRVCoinID, publicKeyBytes)
							if err != nil {
								fmt.Println("len(outs))", len(outs), base58.Base58Check{}.Encode(publicKeyBytes, 0))
								panic(err)
							}
							coinIdx = idxBig.Uint64()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(transactionStateDB, common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(shardID))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = CoinInfo{
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
						outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenID, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
				fmt.Println(tokenID, txHash, len(outs))
				fmt.Println("====================================================\n")
			}
			if tx.GetType() == common.TxCustomTokenPrivacyType || tx.GetType() == common.TxTokenConversionType {
				fmt.Println("\n====================================================")
				fmt.Println(common.ConfidentialAssetID.String(), txHash, tx.IsPrivacy(), tx.GetProof(), tx.GetVersion(), tx.GetMetadataType())
				txToken := tx.(transaction.TransactionToken)
				txTokenData := txToken.GetTxTokenData()
				tokenOuts := txTokenData.TxNormal.GetProof().GetOutputCoins()
				for _, coin := range tokenOuts {
					publicKeyBytes := coin.GetPublicKey().ToBytesS()
					publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
					if publicKeyShardID == byte(shardID) {
						coinIdx := uint64(0)
						tokenStr := txToken.GetTokenID().String()
						if coin.GetVersion() == 2 {
							idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(shardID)], *tx.GetTokenID(), publicKeyBytes)
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							tokenStr = common.ConfidentialAssetID.String()
						} else {
							idxBig, err := statedb.GetCommitmentIndex(transactionStateDB, *txToken.GetTokenID(), coin.GetCommitment().ToBytesS(), byte(shardID))
							if err != nil {
								panic(err)
							}
							coinIdx = idxBig.Uint64()
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
							}
							if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr]; !ok {
								coinV1PubkeyInfo[coin.GetPublicKey().String()][tokenStr] = CoinInfo{
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
						outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), tokenStr, coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
						outCoinList = append(outCoinList, *outCoin)
					}
				}
				fmt.Println(common.ConfidentialAssetID.String(), txHash, len(tokenOuts))
				fmt.Println("====================================================\n")
				if tx.GetTxFee() > 0 {
					outs := tx.GetProof().GetOutputCoins()
					for _, coin := range outs {
						publicKeyBytes := coin.GetPublicKey().ToBytesS()
						publicKeyShardID := common.GetShardIDFromLastByte(publicKeyBytes[len(publicKeyBytes)-1])
						if publicKeyShardID == byte(shardID) {
							coinIdx := uint64(0)
							if coin.GetVersion() == 2 {
								idxBig, err := statedb.GetOTACoinIndex(TransactionStateDB[byte(shardID)], common.PRVCoinID, publicKeyBytes)
								if err != nil {
									panic(err)
								}
								coinIdx = idxBig.Uint64()
							} else {
								idxBig, err := statedb.GetCommitmentIndex(transactionStateDB, common.PRVCoinID, coin.GetCommitment().ToBytesS(), byte(shardID))
								if err != nil {
									panic(err)
								}
								coinIdx = idxBig.Uint64()
								if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()]; !ok {
									coinV1PubkeyInfo[coin.GetPublicKey().String()] = make(map[string]CoinInfo)
								}
								if _, ok := coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()]; !ok {
									coinV1PubkeyInfo[coin.GetPublicKey().String()][common.PRVCoinID.String()] = CoinInfo{
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
							outCoin := NewCoinData(beaconHeight, coinIdx, coin.Bytes(), common.PRVCoinID.String(), coin.GetPublicKey().String(), "", txHash, shardID, int(coin.GetVersion()))
							outCoinList = append(outCoinList, *outCoin)
						}
					}
				}
			}
		}
		if len(outCoinList) > 0 {
			err := DBSaveCoins(outCoinList)
			if err != nil {
				panic(err)
			}
		}
		if len(coinV1PubkeyInfo) > 0 {
			err := DBUpdateCoinV1PubkeyInfo(coinV1PubkeyInfo)
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}
