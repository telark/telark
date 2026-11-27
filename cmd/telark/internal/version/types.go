package version

import (
	"github.com/telark/telark/cmd/telark/internal/kube"
	"github.com/telark/telark/cmd/telark/internal/output"
)

type Info struct {
	Client ClientInfo  `json:"clientVersion"`
	Server *ServerInfo `json:"serverVersion,omitempty"`
}

type ClientInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

type ServerInfo struct {
	Version   string            `json:"version"`
	Namespace string            `json:"namespace"`
	Services  map[string]string `json:"services"`
}

type options struct {
	kube       *kube.Flags
	format     output.Format
	clientOnly bool
}
