package shared

func InitializeIDs[T any](ids *[]T) {
	if ids == nil {
		return
	}

	if *ids == nil {
		*ids = []T{}
	}
}

func ReplaceIDsIfProvided[T any](body map[string]any, key string, newIDs []T, target *[]T) {
	if body == nil {
		return
	}

	if _, providedInBody := body[key]; !providedInBody {
		return
	}

	if newIDs == nil {
		newIDs = []T{}
	}

	*target = newIDs
	body[key] = newIDs
}
