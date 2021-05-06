package shared

import (
	"time"
)

type CoinCache struct {
	Coins           []CoinData
	PRVLastHeight   map[int]uint64
	TokenLastHeight map[int]uint64
	Length          int
	Time            time.Time
}

var coinCache CoinCache

func (cc *CoinCache) Reset() {
	cc.Coins = make([]CoinData, 0)

}

func (cc *CoinCache) Update(coins []CoinData, PRVLastHeight, TokenLastHeight map[int]uint64) {
	cc.Coins = coins
	cc.PRVLastHeight = PRVLastHeight
	cc.TokenLastHeight = TokenLastHeight
	cc.Time = time.Now()
}

func (cc *CoinCache) Read() ([]CoinData, map[int]uint64, map[int]uint64) {
	return cc.Coins, cc.PRVLastHeight, cc.TokenLastHeight
}
