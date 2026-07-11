package category

import (
	categorydata "github.com/telark/data/classification/category"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

func ExtractCategorySpecFromRequestBody(body map[string]any) (*categorydata.Category, error) {
	category, err := sharedutils.ExtractStructFromBodyIgnoringID[categorydata.Category](body)
	if err != nil {
		return nil, err
	}

	return category, nil
}
