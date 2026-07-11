package groupbylabels

import "github.com/telark/data/resources/application"

type GroupByLabelsData struct {
	TotalResources    int                       `json:"totalResources"`
	TotalApplications int                       `json:"totalApplications"`
	Applications      []application.Application `json:"applications"`
}
