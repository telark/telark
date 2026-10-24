package telarkconfig

import "github.com/telark/telark/internal/data/constants"

func (s *BootstrapStatus) Pinned() bool {
	return s != nil && s.PinnedAt != constants.EmptyString
}

// Requests are kept newest first, and only the newest can be open.
func (s *BootstrapStatus) Newest() *BootstrapRequest {
	if s == nil || len(s.Requests) == constants.DefaultInitValue {
		return nil
	}
	return &s.Requests[constants.DefaultInitValue]
}

func (r *BootstrapRequest) Open() bool {
	return r.State == BootstrapStatePending || r.State == BootstrapStateApproved
}
