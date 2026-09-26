package shared

import (
	"context"
	"net/http"
	"slices"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

type NameFetcher struct {
	Dst *[]string
	Fn  func(namespace string) ([]string, error)
}

func RunFetchers(namespace string, fetchers []NameFetcher) error {
	for i := range fetchers {
		f := &fetchers[i]
		if err := AppendNames(f.Dst, func() ([]string, error) { return f.Fn(namespace) }); err != nil {
			return err
		}
	}
	return nil
}

func AppendNames(dst *[]string, fn func() ([]string, error)) error {
	list, err := fn()
	if err != nil {
		return err
	}
	*dst = append(*dst, list...)
	return nil
}

func NamesFromList[T any](
	namespace string,
	getList func(string) ([]T, error),
	getName func(T) string,
) ([]string, error) {
	list, err := getList(namespace)
	if err != nil {
		return nil, err
	}
	names := make([]string, constants.DefaultInitValue, len(list))
	for i := range list {
		names = append(names, getName(list[i]))
	}
	return names, nil
}

// Fails closed: names in excluded namespaces and in the release namespace (the service token
// Secret among them) are never listed, and an unknown excluded list refuses every namespace.
func NamespaceListable(w http.ResponseWriter, r *http.Request, namespace string) bool {
	ctx, cancel := context.WithTimeout(r.Context(), constants.InsightsReadTimeout)
	defer cancel()
	excluded, err := gcfghelper.ExcludedNamespaces(ctx)
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusServiceUnavailable, response.OperationError, err.Error(), nil, err)
		return false
	}
	if slices.Contains(excluded, namespace) || namespace == validation.OwnNamespace() {
		responseutils.LogAndSendResponse(
			w, http.StatusForbidden, response.OperationError, string(constants.ErrNamespaceNotListable), nil, nil,
		)
		return false
	}
	return true
}
