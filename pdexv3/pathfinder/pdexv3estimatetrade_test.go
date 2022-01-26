package pathfinder

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindGoodTradePath(t *testing.T) {
	pc := &PriceCalculator{
		Graph: make(map[string][]Node),
	}

	var maxPathLen uint = 1000
	var pools []*SimplePoolNodeData
	var tokenIDStrSource = "1"
	var tokenIDStrDest = "2"

	pools = make([]*SimplePoolNodeData, 0)

	pools = []*SimplePoolNodeData{
		{
			Token0ID:  "1",
			Token1ID:  "2",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(20),
		},
		{
			Token0ID:  "1",
			Token1ID:  "5",
			Token0Liq: big.NewInt(20),
			Token1Liq: big.NewInt(20),
		},
		{
			Token0ID:  "1",
			Token1ID:  "9",
			Token0Liq: big.NewInt(15),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "2",
			Token1ID:  "3",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "2",
			Token1ID:  "6",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "2",
			Token1ID:  "10",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "3",
			Token1ID:  "4",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "4",
			Token1ID:  "6",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(15),
		},
		{
			Token0ID:  "5",
			Token1ID:  "6",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "5",
			Token1ID:  "8",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "6",
			Token1ID:  "7",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "8",
			Token1ID:  "10",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
		{
			Token0ID:  "9",
			Token1ID:  "10",
			Token0Liq: big.NewInt(10),
			Token1Liq: big.NewInt(10),
		},
	}

	allPaths := pc.findPaths(maxPathLen + 1, pools, tokenIDStrSource, tokenIDStrDest)

	fmt.Printf("Found %d paths\n", len(allPaths))
	for _, path := range allPaths {
		fmt.Println(path)
	}

	assert.Equal(t, 7, len(allPaths), "number of found paths should be 7")
}
