package controllers

import (
	"fmt"
	"strings"
)

// ToLabels constructs a map of a list of labels.
func ToLabels(tags []string) map[string]string {
	result := map[string]string{}
	for _, l := range tags {
		key, value, _ := strings.Cut(l, "=")
		result[key] = value
	}
	return result
}

// ToLabels constructs a list of labels.
func ToTags(labels map[string]string) []string {
	result := []string{}
	for k, v := range labels {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}
