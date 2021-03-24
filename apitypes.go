package main

type API_check_keyimages_request struct {
	Keyimages []string
	ShardID   int
}

type API_submit_otakey_request struct {
	OTAKey       string
	ShardID      int
	BeaconHeight uint64
}

type API_get_random_commitment_request struct {
	Version int
	ShardID int
	TokenID string
	//coinV2 only
	Limit int
	//coinV1 only
	Indexes []uint64
}

type API_respond struct {
	Result interface{}
	Error  *string
}
