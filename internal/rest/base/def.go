package base

import (
	"errors"
	"fmt"

	"github.com/telark/data/logger"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/rest/constants"
)

type (
	Version     string
	Endpoint    string
	Schema      string
	Service     string
	ContentType string
	Port        int
	Method      string
	Host        struct {
		Schema  Schema
		Service Service
		Port    Port
	}
	API struct {
		Host        Host
		Version     Version
		Endpoint    Endpoint
		ContentType ContentType
		Payload     []byte
		Method      Method
	}
)

const (
	HTTP               Schema = "http://"
	HTTPS              Schema = "https://"
	MaxRequestBodySize int64  = 1 << 20 // 1 MB
)

const (
	Configurator Service     = "configurator"
	Exporter     Service     = "exporter"
	Discovery    Service     = "discovery"
	Notifier     Service     = "notifier"
	Enrichment   Service     = "enrichment"
	JSON         ContentType = "application/json"
	Default      Port        = 8080
	HTTPSPort    Port        = 443
	UIPort       Port        = 3000
	V1           Version     = "api/v1"
	Get          Method      = "GET"
	Post         Method      = "POST"
	Update       Method      = "PUT"
	Delete       Method      = "DELETE"
	Patch        Method      = "PATCH"
)

func (api *API) GenerateURL() (string, error) {
	if err := api.Validate(); err != nil {
		return "", fmt.Errorf(string(constants.ErrValidationFailed), err)
	}

	serviceName := GetServiceName(api.Host.Service)

	if (api.Host.Schema == HTTP && api.Host.Port == constants.DefaultHTTPPort) ||
		(api.Host.Schema == HTTPS && api.Host.Port == constants.DefaultHTTPSPort) {
		return fmt.Sprintf("%s%s/%s/%s", api.Host.Schema, serviceName, api.Version,
			api.Endpoint), nil
	}
	return fmt.Sprintf("%s%s:%d/%s/%s", api.Host.Schema, serviceName, api.Host.Port,
		api.Version, api.Endpoint), nil
}

func (api *API) Validate() error {
	if api.Host.Schema == constants.EmptyString {
		return errors.New(string(constants.ErrSchemaIsRequired))
	}
	if api.Host.Service == constants.EmptyString {
		return errors.New(string(constants.ErrServiceIsRequired))
	}
	if api.Version == constants.EmptyString {
		return errors.New(string(constants.ErrVersionIsRequired))
	}
	if api.Endpoint == constants.EmptyString {
		return errors.New(string(constants.ErrEndpointIsRequired))
	}
	return nil
}

func GetServiceName(service Service) string {
	return fmt.Sprintf("%s-%s-service", globalshared.BaseNamespace, service)
}

func GetLogger() *logger.CustomLogger {
	return logger.NewCustomLogger("Rest: ")
}
