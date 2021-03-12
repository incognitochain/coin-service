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

type API_get_coins_respond struct {
}
