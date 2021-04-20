package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
	"io/ioutil"
	"net/http"
)


var URL = URLStaging

func SendPost(url, query string) ([]byte, error) {
	var jsonStr = []byte(query)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, err
		}
		return body, nil
	}
}

func GetListToken(key string, version int) (map[string]InfoJSON, error) {
	method := "getkeyinfo"
	url := ""
	if version == 2 {
		url = fmt.Sprintf("%v/%v?key=%v&version=2", URL, method, key)
	} else {
		url = fmt.Sprintf("%v/%v?key=%v", URL, method, key)
	}
	fmt.Println("Get Key Info URL:", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(fmt.Sprintf("cannot get list outcoin. Error %v", err))
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(fmt.Sprintf("cannot parse body from response text. Error %v", err))
		return nil, err
	}
	tmp := new(KeyInfoJSON)
	if err := json.Unmarshal(respBytes, &tmp); err != nil {
		fmt.Println(fmt.Sprintf("cannot unmarshal json. Error %v", err))
		return nil, err
	}

	listToken := tmp.Result.CoinIndex
	return listToken, nil
}


func GetOutputCoinsFromCS(key, tokenID string, version int) ([]jsonresult.ICoinInfo, int, error) {
	method := "getcoins"
	query := ""
	if version == 1 {
		query = fmt.Sprintf("%v/%v?viewkey=%v&offset=%v&limit=%v&tokenid=%v&version=1",
			URL,
			method,
			key,
			0,
			1000000,
			tokenID)
	} else {
		query = fmt.Sprintf("%v/%v?otakey=%v&offset=%v&limit=%v&tokenid=%v&version=2",
			URL,
			method,
			key,
			0,
			1000000,
			tokenID)
	}

	resp, err := http.Get(query)
	if err != nil {
		fmt.Println(fmt.Sprintf("cannot get list outcoin. Error %v", err))
		return nil, 0, nil
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(fmt.Sprintf("cannot parse body from response text. Error %v", err))
		return nil, 0, nil
	}

	listOutputCoins := make([]jsonresult.ICoinInfo, 0)
	latestIndex := 0

	tmp := new(OutCoinJSON)
	if err := json.Unmarshal(respBytes, &tmp); err != nil {
		err := fmt.Errorf("cannot unmarshal response. Error %v", err)
		fmt.Println(err)
		return nil, 0, err
	} else if tmp.Error != nil {
		err := fmt.Errorf("cannot unmarshal response. Error %v", err)
		fmt.Println(err)
		return nil, 0, err
	} else {
		for _, outCoins := range tmp.Result.Outputs {
			for _, coin := range outCoins {
				out, _, err := jsonresult.NewCoinFromJsonOutCoin(coin)
				if err != nil {
					return nil, 0, err
				}
				listOutputCoins = append(listOutputCoins, out)
			}
		}
		latestIndex = tmp.Result.HighestIndex
	}
	return listOutputCoins, latestIndex, nil
}

func GetOutputCoinsFromRPC(paymentAddress, readOnlyKey string, height int, tokenID string) ([]shared.ICoinInfo, error) {
	query := fmt.Sprintf(`{
		"jsonrpc": "1.0",
		"method": "listoutputcoins",
		"params": [
			0,
			999999,
			[
				{
			  "PaymentAddress": "%s",
			  "ReadOnlyKey":"%s",
			  "StartHeight": %d
				}
			],
		  "%s"
		  ],
		"id": 1
	}`, paymentAddress, readOnlyKey, height, tokenID)

	resp, err := SendPost(URLTestNet, query)
	if err != nil {
		return nil, fmt.Errorf("cannot get list outputcoin from rpc. Error %v", err)
	}

	outputCoins, _, err := ParseCoinFromJsonResponse(resp)
	return outputCoins, err
}


func main() {
	keyWallet, _ := wallet.Base58CheckDeserialize(tmpPrivKeyLists[1])
	keyWallet.KeySet.InitFromPrivateKey(&keyWallet.KeySet.PrivateKey)

	// input: {tokenID, paymentAddress, viewingkey}
	viewingKeyStr := keyWallet.Base58CheckSerialize(wallet.ReadonlyKeyType)
	paymentAddressStr := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
	paymentAddressStr, _ = wallet.GetPaymentAddressV1(paymentAddressStr, false)
	tokenID := "0000000000000000000000000000000000000000000000000000000000000004"

	listOutCoin1, err := GetOutputCoinsFromRPC(paymentAddressStr, viewingKeyStr, 0, tokenID)
	if err != nil {
		fmt.Println("Error:", err)
	}
	listOutCoin2, _, err := GetOutputCoinsFromCS(viewingKeyStr, tokenID, 1)
	if err != nil {
		fmt.Println("Error:", err)
	}

	if len(listOutCoin1) != len(listOutCoin2) {
		fmt.Printf("ERROR: !size %v (FN) - %v (CS)\n", len(listOutCoin1), len(listOutCoin2))
		return
	}
	for _ , coin2 := range listOutCoin2 {
		flag := false
		for _, coin1 := range listOutCoin1 {
			if bytes.Equal(coin1.GetPublicKey().ToBytesS(), coin2.GetPublicKey().ToBytesS()) && bytes.Equal(coin1.GetCommitment().ToBytesS(), coin2.GetCommitment().ToBytesS()) {
				flag = true
				break
			}
		}
		if flag == false {
			fmt.Printf("ERROR: !match coin PK %v - COM %v\n", coin2.GetPublicKey(), coin2.GetCommitment())
		}
	}
	fmt.Println("Done with OK")
}

