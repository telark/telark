package common

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type Fasid struct {
	Name           string       `json:"name"`
	Grouper        string       `json:"grouper"`
	Type           Type         `json:"type"`
	OriginalType   OriginalType `json:"originalType"`
	CreationTime   string       `json:"creationTime"`
	LastUpdateTime string       `json:"lastUpdateTime"`
}

type Type string
type OriginalType string

const (
	GROUPER        Type = "Grouper"
	APP_WORKLOAD   Type = "AppWorkload"
	BATCH_WORKLOAD Type = "BatchWorkload"
	BRIDGE         Type = "Bridge"
)

const (
	NAMESPACE    OriginalType = "Namespace"
	DEPLOYMENT   OriginalType = "Deployment"
	STATEFUL_SET OriginalType = "StatefulSet"
	DAEMON_SET   OriginalType = "DaemonSet"
	SERVICE      OriginalType = "Service"
	JOB          OriginalType = "Job"
	CRON_JOB     OriginalType = "CronJob"
)

type Config struct {
	History []Record `json:"history"`
	Sync    Sync     `json:"sync"`
}

type Sync struct {
	Mode           string `json:"mode"`
	LastUpdateTime string `json:"lastUpdateTime"`
}

type Record struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	CreationTime string `json:"creationTime"`
}

func GenerateName(fasid Fasid) string {
	// Create Name
	name := fmt.Sprintf("%s-%s", fasid.Name, fasid.Type)

	// Convert To Lowercase
	name = strings.ToLower(name)

	// Replace any non-alphanumeric characters (except hyphens) with a hyphen
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "-")

	// Ensure Name Starts & ends with Alphanumeric Character
	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}
