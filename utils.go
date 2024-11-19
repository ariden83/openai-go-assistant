package main

// Fonction utilitaire pour obtenir le maximum entre deux entiers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Fonction utilitaire pour obtenir le minimum entre deux entiers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeDuplicates(strings []string) []string {
	unique := make(map[string]struct{})
	result := []string{}

	for _, str := range strings {
		if _, exists := unique[str]; !exists {
			unique[str] = struct{}{}
			result = append(result, str)
		}
	}

	return result
}
