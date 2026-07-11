package groupbylabels

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	apphistorydiff "github.com/telark/discovery/core/applications/history/diff"
	"github.com/telark/discovery/discovery/derivation"
	discoveryshared "github.com/telark/discovery/discovery/shared"
)

func BuildGroupByLabelsData(withGroups []derivation.ResourceWithGroup) GroupByLabelsData {
	if len(withGroups) == constants.DefaultInitValue {
		return GroupByLabelsData{
			Applications: []application.Application{},
		}
	}

	byApp := make(map[string][]derivation.ResourceWithGroup)
	for _, r := range withGroups {
		byApp[r.Group] = append(byApp[r.Group], r)
	}

	appNames := discoveryshared.OrderedGroupNames(withGroups)
	applications := make([]application.Application, constants.DefaultInitValue, len(appNames))
	for _, name := range appNames {
		resources := byApp[name]
		applications = append(applications, buildApplication(name, resources))
	}

	return GroupByLabelsData{
		TotalResources:    len(withGroups),
		TotalApplications: len(applications),
		Applications:      applications,
	}
}

func buildApplication(name string, resources []derivation.ResourceWithGroup) application.Application {
	nsCounts := make(map[string]int)
	kindCounts := make(map[string]int)
	resItems := make([]application.Resource, constants.DefaultInitValue, len(resources))
	var managedBy string

	for _, r := range resources {
		nsCounts[r.Namespace]++
		kindCounts[r.Kind]++
		resItems = append(resItems, application.Resource{
			Namespace: r.Namespace,
			Kind:      r.Kind,
			Name:      r.Name,
		})
		if managedBy == constants.EmptyString && r.Labels != nil {
			if v := r.Labels[discoveryshared.LabelManagedBy]; v != constants.EmptyString {
				managedBy = v
			}
		}
	}

	primaryNS := discoveryshared.PrimaryNamespaceFromCounts(nsCounts)
	return application.Application{
		Name:          name,
		DisplayName:   discoveryshared.BuildDisplayName(name, primaryNS),
		ResourceCount: len(resources),
		Namespaces: application.Namespaces{
			Total: len(nsCounts),
			Items: discoveryshared.BuildNamespaceItems(nsCounts),
		},
		Managed:         discoveryshared.BuildManaged(managedBy, constants.EmptyString, constants.EmptyString),
		CreatedAt:       constants.EmptyString,
		LastUpdated:     constants.EmptyString,
		ResourceSummary: discoveryshared.ResourceSummaryFromKindCounts(kindCounts),
		Resources:       resItems,
		Snapshots:       []application.ApplicationSnapshot{},
		Metrics:         application.NewApplicationMetrics(),
		History:         apphistorydiff.NewApplicationHistory(),
	}
}
