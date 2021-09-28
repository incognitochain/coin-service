package pathfinder

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/blockchain/pdex/v2utils"
	"github.com/incognitochain/incognito-chain/privacy"

	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"log"
	"math/big"
)

const (
	MaxPaths = 20
)

type PriceCalculator struct {
	Graph map[string][]Node
}

type Node struct {
	TokenIDStr     string
	TokenPoolValue *big.Int
}

func addEdge(
	tokenIDStrSource string,
	tokenIDStrDest string,
	tokenPoolValueDest *big.Int,
	graph map[string][]Node,
) {
	dest := Node{
		TokenIDStr:     tokenIDStrDest,
		TokenPoolValue: tokenPoolValueDest,
	}
	_, found := graph[tokenIDStrSource]
	if !found {
		graph[tokenIDStrSource] = []Node{dest}
	} else {
		isExisted := false
		for _, existedDest := range graph[tokenIDStrSource] {
			if existedDest.TokenIDStr == dest.TokenIDStr {
				isExisted = true
				break
			}
		}
		if !isExisted {
			graph[tokenIDStrSource] = append(graph[tokenIDStrSource], dest)
		}
	}
}

// NOTEs: the built graph would be undirected graph
func (pc *PriceCalculator) buildGraph(
	pools []*shared.Pdexv3PoolPairWithId,
) {
	pc.Graph = make(map[string][]Node)
	for _, pool := range pools {
		addEdge(
			pool.Token0ID().String(),
			pool.Token1ID().String(),
			pool.Token1VirtualAmount(),
			pc.Graph,
		)
		addEdge(
			pool.Token1ID().String(),
			pool.Token0ID().String(),
			pool.Token0VirtualAmount(),
			pc.Graph,
		)
	}
}

func (pc *PriceCalculator) findPath(
	maxPathLen uint,
	tokenIDStrSource string,
	tokenIDStrDest string,
	visited map[string]bool,
	path []string,
	allPaths [][]string,
) [][]string {
	if len(allPaths) == MaxPaths {
		return allPaths
	}
	path = append(path, tokenIDStrSource)
	visited[tokenIDStrSource] = true

	if tokenIDStrSource == tokenIDStrDest {
		allPaths = append(allPaths, path[:])
	} else if len(path) < int(maxPathLen) {
		nodes, found := pc.Graph[tokenIDStrSource]
		if found {
			for _, node := range nodes {
				if visited[node.TokenIDStr] {
					continue
				}
				allPaths = pc.findPath(maxPathLen, node.TokenIDStr, tokenIDStrDest, visited, path, allPaths)
			}
		}
	}
	path = path[:len(path)-1]
	visited[tokenIDStrSource] = false
	return allPaths
}

func (pc *PriceCalculator) findPaths(
	maxPathLen uint,
	pools []*shared.Pdexv3PoolPairWithId,
	tokenIDStrSource string,
	tokenIDStrDest string,
) [][]string {
	pc.buildGraph(pools)

	visited := make(map[string]bool)
	for tokenIDStr := range pc.Graph {
		visited[tokenIDStr] = false
	}
	var path []string
	var allPaths [][]string
	return pc.findPath(
		maxPathLen,
		tokenIDStrSource,
		tokenIDStrDest,
		visited,
		path,
		allPaths,
	)
}

func trade(
	pool *shared.Pdexv3PoolPairWithId,
	poolPairStates map[string]*pdex.PoolPairState,
	tokenIDToBuyStr string,
	tokenIDToSellStr string,
	sellAmount uint64,
) uint64 {
	tokenIDToBuy := pool.Token0ID()
	tokenIDToSell := pool.Token1ID()
	if tokenIDToBuyStr != tokenIDToBuy.String() {
		tokenIDToBuy = pool.Token1ID()
		tokenIDToSell = pool.Token0ID()
	}

	var poolpairs []*rawdbv2.Pdexv3PoolPair
	poolpairs = append(poolpairs, &pool.Pdexv3PoolPair)

	var tradePath []string
	tradePath = append(tradePath, pool.PoolID)

	var receiver privacy.OTAReceiver
	// get relevant, cloned data from state for the trade path
	reserves, lpFeesPerShares, protocolFees, stakingPoolFees, orderbookList, tradeDirections, tokenToBuy, err :=
		pdex.TradePathFromState(tokenIDToSell, tradePath, poolPairStates)

	acceptedMeta, _, err := v2utils.MaybeAcceptTrade(
		sellAmount,
		0,
		tradePath,
		receiver,
		reserves,
		lpFeesPerShares,
		protocolFees,
		stakingPoolFees,
		tradeDirections,
		tokenToBuy,
		0,
		orderbookList,
	)

	if err != nil {
		fmt.Printf("Error calculating trade ammont %s \n", err)
		return 0
	}
	return acceptedMeta.Amount
}

func chooseBestPoolFromAPair(
	pools []*shared.Pdexv3PoolPairWithId,
	poolPairStates map[string]*pdex.PoolPairState,
	tokenIDStrNodeSource string,
	tokenIDStrNodeDest string,
	sellAmt uint64,
) (*shared.Pdexv3PoolPairWithId, uint64) {
	maxReceive := uint64(0)
	var chosenPool *shared.Pdexv3PoolPairWithId
	for _, pool := range pools {
		if (tokenIDStrNodeSource == pool.Token0ID().String() && tokenIDStrNodeDest == pool.Token1ID().String()) || (tokenIDStrNodeSource == pool.Token1ID().String() && tokenIDStrNodeDest == pool.Token0ID().String()) {
			receive := trade(
				pool,
				poolPairStates,
				tokenIDStrNodeDest,
				tokenIDStrNodeSource,
				sellAmt,
			)
			if receive > maxReceive {
				maxReceive = receive
				chosenPool = pool
			}
		}
	}
	return chosenPool, maxReceive
}

func FindGoodTradePath(
	maxPathLen uint,
	pools []*shared.Pdexv3PoolPairWithId,
	poolPairStates map[string]*pdex.PoolPairState,
	tokenIDStrSource string,
	tokenIDStrDest string,
	originalSellAmount uint64,
) ([]*shared.Pdexv3PoolPairWithId, uint64) {

	pc := &PriceCalculator{
		Graph: make(map[string][]Node),
	}

	allPaths := pc.findPaths(maxPathLen, pools, tokenIDStrSource, tokenIDStrDest)

	if len(allPaths) == 0 {
		return []*shared.Pdexv3PoolPairWithId{}, 0
	}

	maxReceive := uint64(0)
	var chosenPath []*shared.Pdexv3PoolPairWithId

	for _, path := range allPaths {
		sellAmt := originalSellAmount

		var pathByPool []*shared.Pdexv3PoolPairWithId

		for i := 0; i < len(path)-1; i++ {
			tokenIDStrNodeSource := path[i]
			tokenIDStrNodeDest := path[i+1]

			pool, receive := chooseBestPoolFromAPair(pools, poolPairStates, tokenIDStrNodeSource, tokenIDStrNodeDest, sellAmt)
			sellAmt = receive
			pathByPool = append(pathByPool, pool)
		}

		if len(pathByPool) == 0 || sellAmt > maxReceive {
			maxReceive = sellAmt
			chosenPath = pathByPool
		}
	}
	return chosenPath, maxReceive
}

func marshalRPCGetState(data json.RawMessage) ([]*shared.Pdexv3PoolPairWithId, map[string]*pdex.PoolPairState) {

	poolPairs := make(map[string]*pdex.PoolPairState)

	err := json.Unmarshal([]byte(data), &poolPairs)
	if err != nil {
		log.Println("Error unmarshal rpc data", err)
		return nil, nil
	}

	var poolPairsArr []*shared.Pdexv3PoolPairWithId

	for poolId, element := range poolPairs {

		var poolPair rawdbv2.Pdexv3PoolPair
		var poolPairWithId shared.Pdexv3PoolPairWithId

		poolPair = element.State()
		poolPairWithId = shared.Pdexv3PoolPairWithId{
			poolPair,
			shared.Pdexv3PoolPairChild{
				PoolID: poolId},
		}

		poolPairsArr = append(poolPairsArr, &poolPairWithId)
	}

	return poolPairsArr, poolPairs
}

func GetPdexv3StateFromRPC() (*shared.Pdexv3GetStateRPCResult, error)  {
	rpcRequestBody := `
		{
		  "id": 1,
		  "jsonrpc": "1.0",
		  "method": "pdexv3_getState",
		  "params": [
			{
			  "BeaconHeight": 0
			}
		  ]
		}
	`

	var responseBodyData shared.Pdexv3GetStateRPCResult
	_, err := shared.RestyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetBody(rpcRequestBody).
		SetResult(&responseBodyData).
		Post(shared.ServiceCfg.FullnodeEndpoint)
	if err != nil {
		log.Printf("Error getting RPC 'pdexv3_getState': %s\n", err.Error())
		return nil, err
	}

	return &responseBodyData, nil
}

func GetPdexv3PoolDataFromRawRPCResult(message json.RawMessage) ([]*shared.Pdexv3PoolPairWithId, map[string]*pdex.PoolPairState, error) {
	pools, poolPairStates := marshalRPCGetState(message)

	return pools, poolPairStates, nil
}
