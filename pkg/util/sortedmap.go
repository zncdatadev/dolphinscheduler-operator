package util

import (
	"maps"
	"slices"
)

type SortedMap map[string]interface{}

func NewSortedMap(mapVal map[string]interface{}) SortedMap {
	return SortedMap(mapVal)
}

// SortedKeys returns a sorted slice of keys in the map.
// The iteration order is sorted alphabetically.
func (s SortedMap) SortedKeys() []string {
	keys := maps.Keys(s)
	return slices.Sorted(keys)
}

// Range iterates over the map and call the given function with each key-value pair.
// The iteration order is sorted alphabetically.
// If the function returns false, iteration is stopped.

// range a sorted map
func (s SortedMap) Range(fn func(key string, value interface{}) bool) {
	keys := s.SortedKeys()
	for _, key := range keys {
		if !fn(key, s[key]) {
			return
		}
	}
}
