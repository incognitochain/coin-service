package database

import (
	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func DBGetDeviceByDeviceQRCode(qrCode string) (*shared.PNodeDevice, error) {
	var result shared.PNodeDevice
	filter := bson.M{"qr_code": bson.M{operator.Eq: qrCode}}
	err := mgm.Coll(&shared.PNodeDevice{}).First(filter, &result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}
