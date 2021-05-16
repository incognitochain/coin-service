package apiservice

type APICheckKeyImagesRequest struct {
	Keyimages []string
	ShardID   int
	Base58    bool
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
	Base58  bool
}

type APIRespond struct {
	Result interface{}
	Error  *string
}

type APILatestTxRespond struct {
	ShardID int
	Time    int
	Height  uint64
	Hash    string
	Type    int
}
