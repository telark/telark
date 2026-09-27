package authz

import "github.com/telark/x-ware/authz"

type grantsEntry struct {
	IssuedAt int64        `json:"issuedAt"`
	Grants   authz.Grants `json:"grants"`
}
