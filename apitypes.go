package main

type API_check_keyimages_request struct {
	Keyimages []string
}

type API_submit_otakey_request struct {
	OTAKey       string
	BeaconHeight uint64
}
