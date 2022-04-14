package apiservice

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/shared"
	jsoniter "github.com/json-iterator/go"
	"github.com/patrickmn/go-cache"
	uuid "github.com/satori/go.uuid"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary
var cachedb *cache.Cache

// var cache *lru.Cache

func StartGinService() {
	log.Println("initiating api-service...")
	cachedb = cache.New(5*time.Minute, 5*time.Minute)
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.GET("/health", APIHealthCheck)

	if shared.ServiceCfg.Mode == shared.QUERYMODE {
		id := uuid.NewV4()
		newServiceConn := coordinator.ServiceConn{
			ServiceName: coordinator.SERVICEGROUP_QUERY,
			ID:          id.String(),
			ReadCh:      make(chan []byte),
			WriteCh:     make(chan []byte),
		}
		coordinatorState.coordinatorConn = &newServiceConn
		coordinatorState.serviceStatus = "pause"
		coordinatorState.pauseService = true
		connectCoordinator(&newServiceConn, shared.ServiceCfg.CoordinatorAddr)
		go willPauseOperation()
		go tokenListWatcher()
		go poolListWatcher()
		r.GET("/getcoinslength", APIGetCoinInfo)
		r.GET("/getcoinspending", APIGetCoinsPending)
		r.GET("/getcoins", APIGetCoins)
		r.GET("/getkeyinfo", APIGetKeyInfo)
		r.POST("/checkkeyimages", APICheckKeyImages)
		r.POST("/getrandomcommitments", APIGetRandomCommitments)
		r.POST("/checktxs", APICheckTXs)
		r.GET("/gettxdetail", APIGetTxDetail)
		r.POST("/gettxsbypubkey", APIGetTxsByPubkey)
		r.GET("/getpendingtxs", APIGetPendingTxs)
		r.GET("/checkpendingtx", APICheckTxPending)

		r.GET("/gettxsbyreceiver", APIGetTxsByReceiver)
		r.POST("/gettxsbysender", APIGetTxsBySender)

		r.GET("/getlatesttx", APIGetLatestTxs)
		r.GET("/getshieldhistory", APIGetShieldHistory)
		r.GET("/getunshieldhistory", APIGetUnshieldHistory)

		r.GET("/gettradehistory", APIGetTradeHistory)
		r.GET("/getpdestate", APIPDEState)
		r.GET("/getcontributehistory", APIGetContributeHistory)
		r.GET("/getwithdrawhistory", APIGetWithdrawHistory)
		r.GET("/getwithdrawfeehistory", APIGetWithdrawFeeHistory)

		// New API format
		//coins
		coinsGroup := r.Group("/coins")
		coinsGroup.GET("/defaulttokens", APIGetDefaultTokens)
		coinsGroup.GET("/tokenlist", APIGetTokenList)
		coinsGroup.POST("/tokeninfo", APIGetTokenInfo)
		coinsGroup.GET("/getcoinspending", APIGetCoinsPending)
		coinsGroup.GET("/getcoins", APIGetCoins)
		coinsGroup.GET("/getkeyinfo", APIGetKeyInfo)
		coinsGroup.POST("/checkkeyimages", APICheckKeyImages)
		coinsGroup.POST("/getrandomcommitments", APIGetRandomCommitments)
		coinsGroup.GET("/getcoinslength", APIGetCoinInfo)

		//tx
		txGroup := r.Group("/txs")
		txGroup.POST("/gettxsbysender", APIGetTxsBySender)
		txGroup.POST("/checktxs", APICheckTXs)
		txGroup.POST("/gettxsbypubkey", APIGetTxsByPubkey)
		txGroup.GET("/gettxsbyreceiver", APIGetTxsByReceiver)
		txGroup.GET("/gettxdetail", APIGetTxDetail)
		txGroup.GET("/getpendingtxs", APIGetPendingTxs)
		txGroup.GET("/checkpendingtx", APICheckTxPending)
		txGroup.GET("/getlatesttx", APIGetLatestTxs)
		//shield
		shieldGroup := r.Group("/shield")
		shieldGroup.GET("/getshieldhistory", APIGetShieldHistory)
		shieldGroup.GET("/getunshieldhistory", APIGetUnshieldHistory)
		shieldGroup.GET("/gettxshield", APIGetTxShield)

		//pdex
		pdex := r.Group("/pdex")

		pdexv1Group := pdex.Group("/v1")
		pdexv1Group.GET("/gettradehistory", APIGetTradeHistory)
		pdexv1Group.GET("/getpdestate", APIPDEState)
		pdexv1Group.GET("/getcontributehistory", APIGetContributeHistory)
		pdexv1Group.GET("/getwithdrawhistory", APIGetWithdrawHistory)
		pdexv1Group.GET("/getwithdrawfeehistory", APIGetWithdrawFeeHistory)

		pdexv3Group := pdex.Group("/v3")
		pdexv3Group.GET("/markettokens", pdexv3{}.ListMarkets)
		pdexv3Group.GET("/listpairs", pdexv3{}.ListPairs)
		pdexv3Group.GET("/tradestatus", pdexv3{}.TradeStatus)
		pdexv3Group.GET("/listpools", pdexv3{}.ListPools)
		pdexv3Group.GET("/poolshare", pdexv3{}.PoolShare)
		pdexv3Group.POST("/poolsdetail", pdexv3{}.PoolsDetail)
		pdexv3Group.POST("/pairsdetail", pdexv3{}.PairsDetail)
		pdexv3Group.GET("/tradehistory", pdexv3{}.TradeHistory)
		pdexv3Group.GET("/tradedetail", pdexv3{}.TradeDetail)
		pdexv3Group.GET("/contributehistory", pdexv3{}.ContributeHistory)
		pdexv3Group.GET("/withdrawhistory", pdexv3{}.WithdrawHistory)
		pdexv3Group.GET("/withdrawfeehistory", pdexv3{}.WithdrawFeeHistory)
		pdexv3Group.GET("/latestorders", pdexv3{}.GetLatestTradeOrders)
		pdexv3Group.GET("/stakingpools", pdexv3{}.StakingPool)
		pdexv3Group.GET("/stakeinfo", pdexv3{}.StakeInfo)
		pdexv3Group.GET("/stakinghistory", pdexv3{}.StakeHistory)
		pdexv3Group.GET("/stakerewardhistory", pdexv3{}.StakeRewardHistory)
		pdexv3Group.GET("/pendingorder", pdexv3{}.PendingOrder)
		pdexv3Group.POST("/rate", pdexv3{}.GetRate)
		pdexv3Group.GET("/getpdestate", pdexv3{}.PDEState)
		pdexv3Group.POST("/pendinglimit", pdexv3{}.PendingLimit)

		//external dependency
		pdexv3Group.GET("/estimatetrade", pdexv3{}.EstimateTrade)
		pdexv3Group.GET("/pricehistory", pdexv3{}.PriceHistory)
		pdexv3Group.GET("/liquidityhistory", pdexv3{}.LiquidityHistory)
		pdexv3Group.GET("/tradevolume-24h", pdexv3{}.TradeVolume24h)
		pdexv3Group.GET("/orderbook", pdexv3{}.GetOrderBook)

		astGroup := pdexv3Group.Group("/assistance")
		astGroup.GET("/top10pairs", APIGetTop10)
		astGroup.GET("/checkrate", APICheckRate)
		astGroup.GET("/plist", APIGetPdecimal)

		deviceGroup := r.Group("/device")
		deviceGroup.GET("/getdevicebyqrcode", APIGetDeviceByQRCode)

	}

	if shared.ServiceCfg.Mode == shared.INDEXERMODE {
		r.POST("/submitotakey", APISubmitOTA)
		r.POST("/rescanotakey", APIRescanOTA)
		r.GET("/indexworker", otaindexer.WorkerRegisterHandler)
		r.GET("/workerstat", otaindexer.GetWorkerStat)
	}

	if shared.ServiceCfg.Mode == shared.COORDINATORMODE {
		coordinatorGroup := r.Group("/coordinator")
		coordinatorGroup.GET("/connectservice", coordinator.ServiceRegisterHandler)
		coordinatorGroup.GET("/backup", coordinator.BackupHandler)
		coordinatorGroup.GET("/backupstatus", coordinator.BackupStatusHandler)
		coordinatorGroup.GET("/servicestat", coordinator.GetServiceStatusHandler)
	}

	err := r.Run("0.0.0.0:" + strconv.Itoa(shared.ServiceCfg.APIPort))
	if err != nil {
		panic(err)
	}
}

func APIHealthCheck(c *gin.Context) {
	//check block time
	//ping pong vs mongo
	status := shared.HEALTH_STATUS_OK
	mongoStatus := shared.MONGO_STATUS_OK
	shardsHeight := make(map[int]string)
	if shared.ServiceCfg.Mode == shared.CHAINSYNCMODE {
		statePrefix := chainsynker.BeaconData
		v, err := chainsynker.Localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
		if err != nil {
			log.Println(err)
		}
		beaconHeight, err := strconv.Atoi(string(v))
		if err != nil {
			beaconHeight = 0
		}
		chainHeight := chainsynker.Localnode.GetBlockchain().BeaconChain.GetBestViewHeight()
		if chainHeight-uint64(beaconHeight) > 5 {
			status = shared.HEALTH_STATUS_NOK
		}
		shardsHeight[-1] = fmt.Sprintf("%v|%v|%v|%v", beaconHeight, chainsynker.Localnode.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetHeight(), chainHeight-uint64(beaconHeight), shared.ServiceCfg.FullnodeData)
		for i := 0; i < chainsynker.Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
			chainheight := chainsynker.Localnode.GetBlockchain().BeaconChain.GetShardBestViewHeight()[byte(i)]
			height, _ := chainsynker.Localnode.GetShardState(i)
			if shared.ServiceCfg.FullnodeData {
				height = chainsynker.Localnode.GetBlockchain().GetBestStateShard(byte(i)).GetHeight()
			}
			statePrefix := fmt.Sprintf("coin-processed-%v", i)
			v, err := chainsynker.Localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
			if err != nil {
				log.Println(err)
			}
			coinHeight, err := strconv.Atoi(string(v))
			if err != nil {
				coinHeight = 0
			}
			if math.Abs(float64(height-chainheight)) > 5 || math.Abs(float64(height-uint64(coinHeight))) > 5 {
				status = shared.HEALTH_STATUS_NOK
			}
			shardsHeight[i] = fmt.Sprintf("%v|%v|%v|%v", coinHeight, height, chainheight, math.Abs(float64(height-chainheight)))
		}
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err := cd.Ping(context.Background(), nil)
	if err != nil {
		status = shared.HEALTH_STATUS_NOK
		mongoStatus = shared.MONGO_STATUS_NOK
	}
	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"mongo":  mongoStatus,
		"chain":  shardsHeight,
	})
}
