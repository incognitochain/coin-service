package main

import "github.com/incognitochain/incognito-chain/privacy"

func OTAKeyFromRaw(b []byte) privacy.OTAKey {
	result := &privacy.OTAKey{}
	result.SetOTASecretKey(b[0:32])
	result.SetPublicSpend(b[32:64])
	return *result
}
