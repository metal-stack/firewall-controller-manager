package firewall

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_ensureTag(t *testing.T) {
	tests := []struct {
		name        string
		currentTags []string
		key         string
		value       string
		want        []string
	}{
		{
			name: "adds a tag that is not already present",
			currentTags: []string{
				"a",
				"b=b",
				"c=",
				"=",
			},
			key:   "d",
			value: "d",
			want: []string{
				"a",
				"b=b",
				"c=",
				"=",
				"d=d",
			},
		},
		{
			name: "updates a tag is present",
			currentTags: []string{
				"a",
				"b=b",
				"c=",
				"d=e",
				"=",
			},
			key:   "d",
			value: "d",
			want: []string{
				"a",
				"b=b",
				"c=",
				"d=d",
				"=",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureTag(tt.currentTags, tt.key, tt.value)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
