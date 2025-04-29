package base

import (
	"errors"
	"fmt"

	"github.com/plsyro/common-pkg/global"
)

type API struct {
	Host        Host
	Version     Version
	Endpoint    Endpoint
	ContentType ContentType
	Payload     []byte
	Method      Method
}

type Host struct {
	Schema  Schema
	Service Service
	Port    Port
}

type Version string
type Endpoint string
type Schema string
type Service string
type ContentType string
type Port int
type Method string

const (
	HTTP  Schema = "http://"
	HTTPS Schema = "https://"
)

const (
	CONFIGURATOR       Service = "configurator"
	EXPORTER           Service = "exporter"
	ADMISSION_OPERATOR Service = "admission-operator"
)

const (
	JSON ContentType = "application/json"
)

const (
	DEFAULT Port = 8080
	UI_PORT Port = 3000
)

const (
	V1 Version = "api/v1"
)

const (
	GET    Method = "GET"
	POST   Method = "POST"
	UPDATE Method = "PUT"
	DELETE Method = "DELETE"
	PATCH  Method = "PATCH"
)

func (api *API) GenerateURL() (string, error) {
	if err := api.Validate(); err != nil {
		return "", err
	}

	serviceName := GetServiceName(api.Host.Service)

	if (api.Host.Schema == HTTP && api.Host.Port == 80) || (api.Host.Schema == HTTPS && api.Host.Port == 443) {
		return fmt.Sprintf("%s%s/%s/%s", api.Host.Schema, serviceName, api.Version, api.Endpoint), nil
	}
	return fmt.Sprintf("%s%s:%d/%s/%s", api.Host.Schema, serviceName, api.Host.Port, api.Version, api.Endpoint), nil
}

func (api *API) Validate() error {
	if api.Host.Schema == "" {
		return errors.New("schema is required")
	}
	if api.Host.Service == "" {
		return errors.New("service is required")
	}
	if api.Version == "" {
		return errors.New("version is required")
	}
	if api.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	return nil
}

func GetServiceName(service Service) string {
	prefix := global.BaseNamespace
	if prefix == "" {
		// Fallback to Default Namespace
		prefix = "default"
	}
	return fmt.Sprintf("%s-%s-service", prefix, service)
}
