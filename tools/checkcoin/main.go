package main

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	pubcoinStr, _, _ := base58.DecodeCheck("16aNdj5ZETKC7BArbAC53bpTNxStKrEZUqpXCdoEsfU6gPzHme")
	txRandomStr, _, _ := base58.DecodeCheck("13XRqvKzJxR8neyuBTrCqdpibNvoDRRbhHAmr224WPvJM7Zn4xBL5Ba6skGjzByJ2eP5c9sAaB4xVg7J96snD4LyeR1g6RfeK7Qu")

	otakeyStr := "14y3Z8HWhYBVz61BrmXFMjiqrEWVtW7TMKpSorGYXjE8sERgBNQZJmuQw45DZfHArh8c5fbikLYB4cczmbd2porUiYebacydG6FDNab"

	wl, err := wallet.Base58CheckDeserialize(otakeyStr)
	if err != nil {
		panic(err)
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		panic(err)
	}

	for i := 0; i < 100000; i++ {
		txRan := coin.NewTxRandom()

		txRan.SetBytes(txRandomStr)

		// _, txOTARandomPoint, index, err1 := c.GetTxRandomDetail()
		txOTARandomPoint, _ := txRan.GetTxOTARandomPoint()
		index, _ := txRan.GetIndex()

		otasecret := wl.KeySet.OTAKey.GetOTASecretKey()
		pubkey := operation.Point{}
		pubkey.FromBytesS(pubcoinStr)

		otapub := wl.KeySet.OTAKey.GetPublicSpend()

		rK := new(operation.Point).ScalarMult(txOTARandomPoint, otasecret)

		hashed := operation.HashToScalar(
			append(rK.ToBytesS(), common.Uint32ToBytes(index)...),
		)

		HnG := new(operation.Point).ScalarMultBase(hashed)
		KCheck := new(operation.Point).Sub(&pubkey, HnG)
		pass := operation.IsPointEqual(KCheck, otapub)
		if !pass {
			panic(22)
		}
	}

}
