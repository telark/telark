package constants

import "time"

const (
	MetricsGetTimeout         = 10 * time.Second
	MetricsListTimeout        = 15 * time.Second
	EventFetchTimeout         = 20 * time.Second
	PodListTimeout            = 30 * time.Second
	PodGetTimeout             = 15 * time.Second
	ServiceGetTimeout         = 20 * time.Second
	ServiceListTimeout        = 30 * time.Second
	NamespaceListTimeout      = 30 * time.Second
	WorkloadGetTimeout        = 30 * time.Second
	WorkloadListTimeout       = 45 * time.Second
	AdmissionGetTimeout       = 15 * time.Second
	AdmissionPatchTimeout     = 15 * time.Second
	AdmissionDeleteTimeout    = 15 * time.Second
	AdmissionCreateTimeout    = 15 * time.Second
	DefaultTimeout            = 30 * time.Second
	ServerVersionTimeout      = 30 * time.Second
	AvailabilityCheckInterval = 5 * time.Minute
	CrdGetTimeout             = 15 * time.Second
	CrdListTimeout            = 30 * time.Second
	CrdPatchTimeout           = 15 * time.Second
	CrdDeleteTimeout          = 15 * time.Second
	CrdCreateTimeout          = 15 * time.Second
)
