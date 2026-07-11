package shared

import "github.com/telark/discovery/constants"

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
