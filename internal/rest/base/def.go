package base

import (
	"fmt"
)

type RestAPI struct {
	Host        Host
	APIVersion  APIVersion
	Endpoint    Endpoint
	ContentType ContentType
	Payload     []byte
	Method      string
}

type Host struct {
	Schema  Schema
	Service Service
	Port    Port
}

type APIVersion string
type Endpoint string
type Schema string
type Service string
type ContentType string
type Port int

const (
	HTTP  Schema = "http://"
	HTTPS Schema = "https://"
)

const (
	CONFIGURATOR       Service = "plsyro-configurator-service"
	EXPORTER           Service = "plsyro-exporter-service"
	ADMISSION_OPERATOR Service = "plsyro-admission-operator-service"
)

const (
	JSON ContentType = "application/json"
)

const (
	DEFAULT Port = 8080
	UI_PORT Port = 3000
)

const (
	V1 APIVersion = "api/v1"
)

func (restApi *RestAPI) GenerateURL() string {
	return fmt.Sprintf("%s%s:%d/%s/%s", restApi.Host.Schema, restApi.Host.Service, restApi.Host.Port, restApi.APIVersion, restApi.Endpoint)
}
