package v2

import (
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func Test_annotationWasRemoved(t *testing.T) {
	tests := []struct {
		name       string
		o          map[string]string
		n          map[string]string
		annotation string
		want       bool
	}{
		{
			name:       "annotation is not present",
			annotation: "c",
			o:          map[string]string{"a": ""},
			n:          map[string]string{"b": ""},
			want:       false,
		},
		{
			name:       "annotation is present",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was added",
			annotation: "c",
			o:          nil,
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was removed",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          nil,
			want:       true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := annotationWasRemoved(event.UpdateEvent{
				ObjectOld: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.o,
					},
				},
				ObjectNew: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.n,
					},
				},
			}, tt.annotation); got != tt.want {
				t.Errorf("annotationWasRemoved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_annotationWasAdded(t *testing.T) {
	tests := []struct {
		name       string
		o          map[string]string
		n          map[string]string
		annotation string
		want       bool
	}{
		{
			name:       "annotation is not present",
			annotation: "c",
			o:          map[string]string{"a": ""},
			n:          map[string]string{"b": ""},
			want:       false,
		},
		{
			name:       "annotation is present",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was added",
			annotation: "c",
			o:          nil,
			n:          map[string]string{"c": ""},
			want:       true,
		},
		{
			name:       "annotation was removed",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          nil,
			want:       false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := annotationWasAdded(event.UpdateEvent{
				ObjectOld: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.o,
					},
				},
				ObjectNew: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.n,
					},
				},
			}, tt.annotation); got != tt.want {
				t.Errorf("annotationWasAdded() = %v, want %v", got, tt.want)
			}
		})
	}
}
