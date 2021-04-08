package main

import (
	"fmt"

	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	wl, _ := wallet.Base58CheckDeserialize("112t8rnZDRztVgPjbYQiXS7mJgaTzn66NvHD7Vus2SrhSAY611AzADsPFzKjKQCKWTgbkgYrCPo9atvSMoCf9KT23Sc7Js9RKhzbNJkxpJU6")
	key := wl.Base58CheckSerialize(wallet.OTAKeyType)
	fmt.Println(key)
}
