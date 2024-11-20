package main

import "fmt"

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

// ArrayStringFlag are defined for string flags that may have multiple values.
type ArrayStringFlag []string

// Returns the concatenated string representation of the array of flags.
func (f *ArrayStringFlag) String() string {
	return fmt.Sprintf("%v", *f)
}

// Get returns an empty interface that may be type-asserted to the underlying
// value of type bool, string, etc.
func (f *ArrayStringFlag) Get() interface{} {
	return ""
}

// Set appends value the array of flags.
func (f *ArrayStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
