package detector

import "sync"

type Detector struct {
	Lck      sync.RWMutex
	Services map[string]ServiceCrashRecorder
}

type ServiceCrashRecorder struct {
	LostConnectionCount int
	KnownCrash          []CrashReason
	UnknownCrash        []CrashReason
}

type CrashReason struct {
	ServiceID string
	Reason    string
	Time      int64
}
