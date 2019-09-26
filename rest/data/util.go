package data

func containsTags(subset, tags []string) bool {
	if len(subset) == 0 {
		return true
	}

	tagMap := map[string]bool{}

	for _, tag := range tags {
		tagMap[tag] = true
	}

	for _, tag := range subset {
		if tagMap[tag] {
			return true
		}
	}

	return false
}
