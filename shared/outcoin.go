package shared

import (
	"encoding/base64"
	"errors"
	"log"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
)

type ICoinInfo interface {
	GetVersion() uint8
	GetCommitment() *privacy.Point
	GetInfo() []byte
	GetPublicKey() *privacy.Point
	GetValue() uint64
	GetKeyImage() *privacy.Point
	GetRandomness() *privacy.Scalar
	GetShardID() (uint8, error)
	GetSNDerivator() *privacy.Scalar
	GetCoinDetailEncrypted() []byte
	IsEncrypted() bool
	GetTxRandom() *coin.TxRandom
	GetSharedRandom() *privacy.Scalar
	GetSharedConcealRandom() *privacy.Scalar
	GetAssetTag() *privacy.Point
}

type OutCoinV2 struct {
	Version              string `json:"Version"`
	Index                string `json:"Index"`
	PublicKey            string `json:"PublicKey"`
	Commitment           string `json:"Commitment"`
	SNDerivator          string `json:"SNDerivator"`
	KeyImage             string `json:"KeyImage"`
	Randomness           string `json:"Randomness"`
	Value                string `json:"Value"`
	Info                 string `json:"Info"`
	SharedRandom         string `json:"SharedRandom"`
	SharedConcealRandom  string `json:"SharedConcealRandom"`
	TxRandom             string `json:"TxRandom"`
	CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
	AssetTag             string `json:"AssetTag"`
	TxHash               string `json:"TxHash"`
}

type OutCoinV1 struct {
	Version     string `json:"Version"`
	Index       string `json:"Index"`
	PublicKey   string `json:"PublicKey"`
	Commitment  string `json:"Commitment"`
	SNDerivator string `json:"SNDerivator"`
	KeyImage    string `json:"KeyImage"`
	Randomness  string `json:"Randomness"`
	Value       string `json:"Value"`
	Info        string `json:"Info"`
	// SharedRandom         string `json:"SharedRandom"`
	// SharedConcealRandom  string `json:"SharedConcealRandom"`
	// TxRandom             string `json:"TxRandom"`
	CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
	// AssetTag             string `json:"AssetTag"`
	TxHash string `json:"TxHash"`
}

func NewOutcoinV1FromInterface(data interface{}) (*OutCoinV1, error) {
	outcoin := OutCoinV1{}
	temp, err := json.Marshal(data)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	err = json.Unmarshal(temp, &outcoin)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	return &outcoin, nil
}

func NewOutCoinV2(outCoin ICoinInfo, base58Fmt bool) OutCoinV2 {
	keyImage := ""
	if outCoin.GetKeyImage() != nil && !outCoin.GetKeyImage().IsIdentity() {
		if base58Fmt {
			keyImage = base58.Base58Check{}.Encode(outCoin.GetKeyImage().ToBytesS(), common.ZeroByte)
		} else {
			keyImage = base64.StdEncoding.EncodeToString(outCoin.GetKeyImage().ToBytesS())
		}
	}

	publicKey := ""
	if outCoin.GetPublicKey() != nil {
		if base58Fmt {
			publicKey = base58.Base58Check{}.Encode(outCoin.GetPublicKey().ToBytesS(), common.ZeroByte)
		} else {
			publicKey = base64.StdEncoding.EncodeToString(outCoin.GetPublicKey().ToBytesS())
		}
	}

	commitment := ""
	if outCoin.GetCommitment() != nil {
		if base58Fmt {
			commitment = base58.Base58Check{}.Encode(outCoin.GetCommitment().ToBytesS(), common.ZeroByte)
		} else {
			commitment = base64.StdEncoding.EncodeToString(outCoin.GetCommitment().ToBytesS())
		}
	}

	snd := ""
	if outCoin.GetSNDerivator() != nil {
		if base58Fmt {
			snd = base58.Base58Check{}.Encode(outCoin.GetSNDerivator().ToBytesS(), common.ZeroByte)
		} else {
			snd = base64.StdEncoding.EncodeToString(outCoin.GetSNDerivator().ToBytesS())
		}
	}

	randomness := ""
	if outCoin.GetRandomness() != nil {
		if base58Fmt {
			randomness = base58.Base58Check{}.Encode(outCoin.GetRandomness().ToBytesS(), common.ZeroByte)
		} else {
			randomness = base64.StdEncoding.EncodeToString(outCoin.GetRandomness().ToBytesS())
		}
	}

	result := OutCoinV2{
		Version:     strconv.FormatUint(uint64(outCoin.GetVersion()), 10),
		PublicKey:   publicKey,
		Value:       strconv.FormatUint(outCoin.GetValue(), 10),
		Commitment:  commitment,
		SNDerivator: snd,
		KeyImage:    keyImage,
		Randomness:  randomness,
	}

	if len(outCoin.GetInfo()) != 0 {
		if base58Fmt {
			result.Info = base58.Base58Check{}.Encode(outCoin.GetInfo(), common.ZeroByte)
		} else {
			result.Info = base64.StdEncoding.EncodeToString(outCoin.GetInfo())
		}
	}

	if outCoin.GetCoinDetailEncrypted() != nil {
		if base58Fmt {
			result.CoinDetailsEncrypted = base58.Base58Check{}.Encode(outCoin.GetCoinDetailEncrypted(), common.ZeroByte)
		} else {
			result.CoinDetailsEncrypted = base64.StdEncoding.EncodeToString(outCoin.GetCoinDetailEncrypted())
		}
	}

	if outCoin.GetSharedRandom() != nil {
		if base58Fmt {
			result.SharedRandom = base58.Base58Check{}.Encode(outCoin.GetSharedRandom().ToBytesS(), common.ZeroByte)
		} else {
			result.SharedRandom = base64.StdEncoding.EncodeToString(outCoin.GetSharedRandom().ToBytesS())
		}
	}
	if outCoin.GetSharedConcealRandom() != nil {
		if base58Fmt {
			result.SharedRandom = base58.Base58Check{}.Encode(outCoin.GetSharedConcealRandom().ToBytesS(), common.ZeroByte)
		} else {
			result.SharedRandom = base64.StdEncoding.EncodeToString(outCoin.GetSharedConcealRandom().ToBytesS())
		}
	}
	if outCoin.GetTxRandom() != nil {
		if base58Fmt {
			result.TxRandom = base58.Base58Check{}.Encode(outCoin.GetTxRandom().Bytes(), common.ZeroByte)
		} else {
			result.TxRandom = base64.StdEncoding.EncodeToString(outCoin.GetTxRandom().Bytes())
		}
	}
	if outCoin.GetAssetTag() != nil {
		if base58Fmt {
			result.AssetTag = base58.Base58Check{}.Encode(outCoin.GetAssetTag().ToBytesS(), common.ZeroByte)
		} else {
			result.AssetTag = base64.StdEncoding.EncodeToString(outCoin.GetAssetTag().ToBytesS())
		}
	}

	return result
}

func NewOutCoinV1(outCoin ICoinInfo, base58Fmt bool) OutCoinV1 {
	keyImage := ""
	if outCoin.GetKeyImage() != nil && !outCoin.GetKeyImage().IsIdentity() {
		if base58Fmt {
			keyImage = base58.Base58Check{}.Encode(outCoin.GetKeyImage().ToBytesS(), common.ZeroByte)
		} else {
			keyImage = base64.StdEncoding.EncodeToString(outCoin.GetKeyImage().ToBytesS())
		}
	}

	publicKey := ""
	if outCoin.GetPublicKey() != nil {
		if base58Fmt {
			publicKey = base58.Base58Check{}.Encode(outCoin.GetPublicKey().ToBytesS(), common.ZeroByte)
		} else {
			publicKey = base64.StdEncoding.EncodeToString(outCoin.GetPublicKey().ToBytesS())
		}
	}

	commitment := ""
	if outCoin.GetCommitment() != nil {
		if base58Fmt {
			commitment = base58.Base58Check{}.Encode(outCoin.GetCommitment().ToBytesS(), common.ZeroByte)
		} else {
			commitment = base64.StdEncoding.EncodeToString(outCoin.GetCommitment().ToBytesS())
		}
	}

	snd := ""
	if outCoin.GetSNDerivator() != nil {
		if base58Fmt {
			snd = base58.Base58Check{}.Encode(outCoin.GetSNDerivator().ToBytesS(), common.ZeroByte)
		} else {
			snd = base64.StdEncoding.EncodeToString(outCoin.GetSNDerivator().ToBytesS())
		}
	}

	randomness := ""
	if outCoin.GetRandomness() != nil {
		if base58Fmt {
			randomness = base58.Base58Check{}.Encode(outCoin.GetRandomness().ToBytesS(), common.ZeroByte)
		} else {
			randomness = base64.StdEncoding.EncodeToString(outCoin.GetRandomness().ToBytesS())
		}
	}

	result := OutCoinV1{
		Version:     strconv.FormatUint(uint64(outCoin.GetVersion()), 10),
		PublicKey:   publicKey,
		Value:       strconv.FormatUint(outCoin.GetValue(), 10),
		Commitment:  commitment,
		SNDerivator: snd,
		KeyImage:    keyImage,
		Randomness:  randomness,
	}

	if len(outCoin.GetInfo()) != 0 {
		if base58Fmt {
			result.Info = base58.Base58Check{}.Encode(outCoin.GetInfo(), common.ZeroByte)
		} else {
			result.Info = base64.StdEncoding.EncodeToString(outCoin.GetInfo())
		}
	}

	return result
}

func NewCoinFromJsonOutCoinV2(jsonOutCoin OutCoinV2) (ICoinInfo, *big.Int, error) {
	var keyImage, pubkey, cm *privacy.Point
	var randomness *privacy.Scalar
	var info []byte
	var err error
	var idx *big.Int
	var sharedRandom, sharedConcealRandom *privacy.Scalar
	var txRandom *coin.TxRandom
	// var coinDetailEncrypted *privacy.HybridCipherText
	var assetTag *privacy.Point

	value, ok := math.ParseUint64(jsonOutCoin.Value)
	if !ok {
		return nil, nil, errors.New("Cannot parse value")
	}

	if len(jsonOutCoin.KeyImage) == 0 {
		keyImage = nil
	} else {
		keyImageInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.KeyImage)
		if err != nil {
			return nil, nil, err
		}
		keyImage, err = new(privacy.Point).FromBytesS(keyImageInBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.Commitment) == 0 {
		cm = nil
	} else {
		cmInbytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Commitment)
		if err != nil {
			return nil, nil, err
		}
		cm, err = new(privacy.Point).FromBytesS(cmInbytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.PublicKey) == 0 {
		pubkey = nil
	} else {
		pubkeyInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.PublicKey)
		if err != nil {
			return nil, nil, err
		}
		pubkey, err = new(privacy.Point).FromBytesS(pubkeyInBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.Randomness) == 0 {
		randomness = nil
	} else {
		randomnessInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Randomness)
		if err != nil {
			return nil, nil, err
		}
		randomness = new(privacy.Scalar).FromBytesS(randomnessInBytes)
	}

	// if len(jsonOutCoin.SNDerivator) == 0 {
	// 	snd = nil
	// } else {
	// 	sndInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SNDerivator)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	snd = new(privacy.Scalar).FromBytesS(sndInBytes)
	// }

	if len(jsonOutCoin.Info) == 0 {
		info = []byte{}
	} else {
		info, _, err = base58.Base58Check{}.Decode(jsonOutCoin.Info)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.SharedRandom) == 0 {
		sharedRandom = nil
	} else {
		sharedRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SharedRandom)
		if err != nil {
			return nil, nil, err
		}
		sharedRandom = new(privacy.Scalar).FromBytesS(sharedRandomInBytes)
	}

	if len(jsonOutCoin.SharedConcealRandom) == 0 {
		sharedRandom = nil
	} else {
		sharedConcealRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SharedConcealRandom)
		if err != nil {
			return nil, nil, err
		}
		sharedConcealRandom = new(privacy.Scalar).FromBytesS(sharedConcealRandomInBytes)
	}

	if len(jsonOutCoin.TxRandom) == 0 {
		sharedRandom = nil
	} else {
		txRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.TxRandom)
		if err != nil {
			return nil, nil, err
		}
		txRandom = new(coin.TxRandom)
		err = txRandom.SetBytes(txRandomInBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.AssetTag) == 0 {
		assetTag = nil
	} else {
		assetTagInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.AssetTag)
		if err != nil {
			return nil, nil, err
		}
		assetTag, err = new(privacy.Point).FromBytesS(assetTagInBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.Index) == 0 {
		idx = nil
	} else {
		idxInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Index)
		if err != nil {
			return nil, nil, err
		}
		idx = new(big.Int).SetBytes(idxInBytes)
	}

	// if jsonOutCoin.Version == "1" {
	// 	pCoinV1 := new(coin.PlainCoinV1).Init()

	// 	pCoinV1.SetRandomness(randomness)
	// 	pCoinV1.SetPublicKey(pubkey)
	// 	pCoinV1.SetCommitment(cm)
	// 	pCoinV1.SetSNDerivator(snd)
	// 	pCoinV1.SetKeyImage(keyImage)
	// 	pCoinV1.SetInfo(info)
	// 	pCoinV1.SetValue(value)

	// 	if len(jsonOutCoin.CoinDetailsEncrypted) != 0 {
	// 		coinDetailEncryptedInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.CoinDetailsEncrypted)
	// 		if err != nil {
	// 			return nil, nil, err
	// 		}
	// 		coinDetailEncrypted = new(privacy.HybridCipherText)
	// 		err = coinDetailEncrypted.SetBytes(coinDetailEncryptedInBytes)
	// 		if err != nil {
	// 			return nil, nil, err
	// 		}

	// 		coinV1 := new(coin.CoinV1).Init()
	// 		coinV1.CoinDetails = pCoinV1
	// 		coinV1.CoinDetailsEncrypted = coinDetailEncrypted

	// 		return coinV1, idx, nil
	// 	}

	// 	return pCoinV1, idx, nil
	// } else
	if jsonOutCoin.Version == "2" {
		coinV2 := new(coin.CoinV2).Init()
		if len(jsonOutCoin.CoinDetailsEncrypted) != 0 {
			coinDetailEncryptedInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.CoinDetailsEncrypted)
			if err != nil {
				return nil, nil, err
			}
			amountEncrypted := new(privacy.Scalar).FromBytesS(coinDetailEncryptedInBytes)
			coinV2.SetAmount(amountEncrypted)
		} else {
			coinV2.SetValue(value)
		}

		coinV2.SetRandomness(randomness)
		coinV2.SetPublicKey(pubkey)
		coinV2.SetCommitment(cm)
		coinV2.SetKeyImage(keyImage)
		coinV2.SetInfo(info)
		coinV2.SetAssetTag(assetTag)
		coinV2.SetSharedRandom(sharedRandom)
		coinV2.SetSharedConcealRandom(sharedConcealRandom)
		coinV2.SetTxRandom(txRandom)

		return coinV2, idx, nil
	}

	return nil, nil, errors.New("cannot find coin version")
}

func NewCoinFromJsonOutCoinV1(jsonOutCoin OutCoinV1) (ICoinInfo, *big.Int, error) {
	var keyImage, pubkey, cm *privacy.Point
	var snd, randomness *privacy.Scalar
	var info []byte
	var err error
	var idx *big.Int

	value, ok := math.ParseUint64(jsonOutCoin.Value)
	if !ok {
		return nil, nil, errors.New("Cannot parse value")
	}

	if len(jsonOutCoin.KeyImage) == 0 {
		keyImage = nil
	} else {
		keyImageInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.KeyImage)
		if err != nil {
			return nil, nil, err
		}
		keyImage, err = new(privacy.Point).FromBytesS(keyImageInBytes)
		if err != nil {
			return nil, nil, err
		}
	}

	if len(jsonOutCoin.Commitment) == 0 {
		cm = nil
	} else {
		cmInbytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Commitment)
		if err != nil {
			return nil, nil, err
		}
		cm, err = new(privacy.Point).FromBytesS(cmInbytes)
		if err != nil {
			return nil, nil, err
		}
	}

	// if len(jsonOutCoin.PublicKey) == 0 {
	// 	pubkey = nil
	// } else {
	// 	pubkeyInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.PublicKey)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	pubkey, err = new(privacy.Point).FromBytesS(pubkeyInBytes)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// }

	if len(jsonOutCoin.Randomness) == 0 {
		randomness = nil
	} else {
		randomnessInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Randomness)
		if err != nil {
			return nil, nil, err
		}
		randomness = new(privacy.Scalar).FromBytesS(randomnessInBytes)
	}

	if len(jsonOutCoin.SNDerivator) == 0 {
		snd = nil
	} else {
		sndInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SNDerivator)
		if err != nil {
			return nil, nil, err
		}
		snd = new(privacy.Scalar).FromBytesS(sndInBytes)
	}

	if len(jsonOutCoin.Info) == 0 {
		info = []byte{}
	} else {
		info, _, err = base58.Base58Check{}.Decode(jsonOutCoin.Info)
		if err != nil {
			return nil, nil, err
		}
	}

	// if len(jsonOutCoin.SharedRandom) == 0 {
	// 	sharedRandom = nil
	// } else {
	// 	sharedRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SharedRandom)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	sharedRandom = new(privacy.Scalar).FromBytesS(sharedRandomInBytes)
	// }

	// if len(jsonOutCoin.SharedConcealRandom) == 0 {
	// 	sharedRandom = nil
	// } else {
	// 	sharedConcealRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.SharedConcealRandom)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	sharedConcealRandom = new(privacy.Scalar).FromBytesS(sharedConcealRandomInBytes)
	// }

	// if len(jsonOutCoin.TxRandom) == 0 {
	// 	sharedRandom = nil
	// } else {
	// 	txRandomInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.TxRandom)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	txRandom = new(coin.TxRandom)
	// 	err = txRandom.SetBytes(txRandomInBytes)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// }

	// if len(jsonOutCoin.AssetTag) == 0 {
	// 	assetTag = nil
	// } else {
	// 	assetTagInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.AssetTag)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// 	assetTag, err = new(privacy.Point).FromBytesS(assetTagInBytes)
	// 	if err != nil {
	// 		return nil, nil, err
	// 	}
	// }

	if len(jsonOutCoin.Index) == 0 {
		idx = nil
	} else {
		idxInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.Index)
		if err != nil {
			return nil, nil, err
		}
		idx = new(big.Int).SetBytes(idxInBytes)
	}

	if jsonOutCoin.Version == "1" {
		pCoinV1 := new(coin.PlainCoinV1).Init()

		pCoinV1.SetRandomness(randomness)
		pCoinV1.SetPublicKey(pubkey)
		pCoinV1.SetCommitment(cm)
		pCoinV1.SetSNDerivator(snd)
		pCoinV1.SetKeyImage(keyImage)
		pCoinV1.SetInfo(info)
		pCoinV1.SetValue(value)

		if len(jsonOutCoin.CoinDetailsEncrypted) != 0 {
			coinDetailEncryptedInBytes, _, err := base58.Base58Check{}.Decode(jsonOutCoin.CoinDetailsEncrypted)
			if err != nil {
				return nil, nil, err
			}
			coinDetailEncrypted := new(privacy.HybridCipherText)
			err = coinDetailEncrypted.SetBytes(coinDetailEncryptedInBytes)
			if err != nil {
				return nil, nil, err
			}

			coinV1 := new(coin.CoinV1).Init()
			coinV1.CoinDetails = pCoinV1
			coinV1.CoinDetailsEncrypted = coinDetailEncrypted

			return coinV1, idx, nil
		}

		return pCoinV1, idx, nil
	}

	return nil, nil, errors.New("cannot find coin version")
}
