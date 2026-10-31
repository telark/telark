package shared

import (
	"net/http"

	"github.com/telark/telark/internal/data/text"
)

type (
	ResponseCapture struct {
		http.ResponseWriter
		status int
	}

	UpstreamError struct {
		Status int
		Err    error
	}

	// A MaxItems above zero makes the field a list whose items each follow the rule.
	TextField struct {
		text.Rule
		MaxItems int
	}
)
