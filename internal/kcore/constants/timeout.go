package constants

import "time"

const (
	MetricsGetTimeout         = 10 * time.Second
	MetricsListTimeout        = 20 * time.Second
	EventFetchTimeout         = 20 * time.Second
	PodListTimeout            = 30 * time.Second
	PodGetTimeout             = 10 * time.Second
	ServiceGetTimeout         = 10 * time.Second
	ServiceListTimeout        = 30 * time.Second
	ResourceListTimeout       = 30 * time.Second
	GroupSearchTimeout        = 90 * time.Second
	NamespaceListTimeout      = 30 * time.Second
	NamespaceGetTimeout       = 10 * time.Second
	WorkloadGetTimeout        = 20 * time.Second
	WorkloadListTimeout       = 45 * time.Second
	AdmissionGetTimeout       = 15 * time.Second
	AdmissionPatchTimeout     = 15 * time.Second
	AdmissionDeleteTimeout    = 15 * time.Second
	AdmissionCreateTimeout    = 15 * time.Second
	DefaultTimeout            = 30 * time.Second
	ServerVersionTimeout      = 30 * time.Second
	AvailabilityCheckInterval = 5 * time.Minute
	CrdGetTimeout             = 10 * time.Second
	CrdListTimeout            = 30 * time.Second
	CrdPatchTimeout           = 15 * time.Second
	CrdDeleteTimeout          = 15 * time.Second
	CrdCreateTimeout          = 12 * time.Second
)
