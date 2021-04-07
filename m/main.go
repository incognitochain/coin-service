package main

import (
	"fmt"

	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	wl, _ := wallet.Base58CheckDeserialize("112t8roafGgHL1rhAP9632Yef3sx5k8xgp8cwK4MCJsCL1UWcxXvpzg97N4dwvcD735iKf31Q2ZgrAvKfVjeSUEvnzKJyyJD3GqqSZdxN4or")
	key := wl.Base58CheckSerialize(wallet.OTAKeyType)
	fmt.Println(key)
}
