package protection

import "github.com/telark/discovery/core/plans/protection"

var globalService *protection.Service

func InitService(svc *protection.Service) {
	globalService = svc
}
