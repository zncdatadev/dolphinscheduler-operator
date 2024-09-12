package util

import (
	"maps"
	"slices"
	"strings"
)

func ToProperties(properties map[string]string) string {
	var sb strings.Builder
	//sort properties
	keys := maps.Keys(properties)
	sortKeys := slices.Sorted(keys)

	for _, key := range sortKeys {
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(properties[key])
		sb.WriteString("\n")
	}
	return sb.String()
}
