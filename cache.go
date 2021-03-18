package main

import (
	"time"
)

type CoinCache struct {
	Coins           []CoinData
	PRVLastHeight   uint64
	TokenLastHeight uint64
	Length          int
	Time            time.Time
}

var coinCache CoinCache

func (cc *CoinCache) Reset() {
	cc.Coins = make([]CoinData, 0)

}

func (cc *CoinCache) Update(coins []CoinData, PRVLastHeight, TokenLastHeight uint64) {
	cc.Coins = coins
	cc.PRVLastHeight = PRVLastHeight
	cc.TokenLastHeight = TokenLastHeight
	cc.Time = time.Now()
}

func (cc *CoinCache) Read() ([]CoinData, uint64, uint64) {
	return cc.Coins, cc.PRVLastHeight, cc.TokenLastHeight
}
