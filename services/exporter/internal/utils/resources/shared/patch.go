package shared

import "time"

func AddLastUpdateDateToPatchBody(body map[string]any) {
	if body == nil {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	body["lastUpdateDate"] = now
}
