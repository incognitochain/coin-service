package main

import (
	"fmt"

	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	wl, _ := wallet.Base58CheckDeserialize("12RvqY93jN5Vnii82rc1xbYqVtLNYBZLC92JA9UwHvvz9sDmJeZYouh6U1uSMFpZTdUZqeXuwHLYoxPaYMgWnUyW9QhpERm2L4NcAiu")
	fmt.Println(base58.EncodeCheck(wl.KeySet.PaymentAddress.Pk))

}
