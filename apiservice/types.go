package apiservice

type APICheckKeyImagesRequest struct {
	Keyimages []string
	ShardID   int
}

type APICheckTxRequest struct {
	Txs     []string
	ShardID int
}

type APIParseTokenidRequest struct {
	OTARandoms []string
	AssetTags  []string
	OTAKey     string
}
type APIGetTxsRequest struct {
	Publickey string
	Skip      int
}

type APISubmitOTAkeyRequest struct {
	OTAKey       string
	ShardID      int
	BeaconHeight uint64
}

type APIGetRandomCommitmentRequest struct {
	Version int
	ShardID int
	TokenID string
	//coinV2 only
	Limit int
	//coinV1 only
	Indexes []uint64
}

type APIRespond struct {
	Result interface{}
	Error  *string
}
