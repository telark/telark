package policies

import "fmt"

var registry = map[string]TemplateRenderer{}

func Register(r TemplateRenderer) {
	if _, exists := registry[r.TemplateID()]; exists {
		panic(fmt.Sprintf("policies: duplicate template renderer registered for ID %q", r.TemplateID()))
	}
	registry[r.TemplateID()] = r
}

func GetRenderer(templateID string) (TemplateRenderer, bool) {
	r, ok := registry[templateID]
	return r, ok
}
