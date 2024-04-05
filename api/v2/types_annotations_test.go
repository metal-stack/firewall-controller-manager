package v2

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestIsAnnotationPresent(t *testing.T) {
	tests := []struct {
		name string
		o    client.Object
		key  string
		want bool
	}{
		{
			name: "not present",
			o:    &Firewall{},
			key:  "a",
			want: false,
		},
		{
			name: "present",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "",
					},
				},
			},
			key:  "a",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAnnotationPresent(tt.o, tt.key); got != tt.want {
				t.Errorf("IsAnnotationPresent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAnnotationIsTrue(t *testing.T) {
	tests := []struct {
		name string
		o    client.Object
		key  string
		want bool
	}{
		{
			name: "not present",
			o:    &Firewall{},
			key:  "a",
			want: false,
		},
		{
			name: "not true",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "foo",
					},
				},
			},
			key:  "a",
			want: false,
		},
		{
			name: "true",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "true",
					},
				},
			},
			key:  "a",
			want: true,
		},
		{
			name: "different variant of true is also allowed",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "1",
					},
				},
			},
			key:  "a",
			want: true,
		},
		{
			name: "false",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "false",
					},
				},
			},
			key:  "a",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAnnotationTrue(tt.o, tt.key); got != tt.want {
				t.Errorf("IsAnnotationTrue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAnnotationIsFalse(t *testing.T) {
	tests := []struct {
		name string
		o    client.Object
		key  string
		want bool
	}{
		{
			name: "not present",
			o:    &Firewall{},
			key:  "a",
			want: false,
		},
		{
			name: "not true",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "foo",
					},
				},
			},
			key:  "a",
			want: false,
		},
		{
			name: "true",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "true",
					},
				},
			},
			key:  "a",
			want: false,
		},
		{
			name: "false",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "false",
					},
				},
			},
			key:  "a",
			want: true,
		},
		{
			name: "different variant of false is also allowed",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"a": "0",
					},
				},
			},
			key:  "a",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAnnotationFalse(tt.o, tt.key); got != tt.want {
				t.Errorf("IsAnnotationFalse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name    string
		o       *Firewall
		key     string
		value   string
		wantErr error
		want    *Firewall
	}{
		{
			name: "add",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "0",
				},
			},
			key:     "test",
			value:   "true",
			wantErr: nil,
			want: &Firewall{
				TypeMeta: v1.TypeMeta{
					Kind:       "Firewall",
					APIVersion: "firewall.metal-stack.io/v2",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "1",
					Annotations: map[string]string{
						"test": "true",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.o).Build()

			err := AddAnnotation(ctx, c, tt.o, tt.key, tt.value)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			updated := &Firewall{}
			err = c.Get(ctx, client.ObjectKeyFromObject(tt.o), updated)
			require.NoError(t, err)

			if diff := cmp.Diff(updated, tt.want); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}

func TestRemoveAnnotation(t *testing.T) {
	scheme := runtime.NewScheme()
	err := AddToScheme(scheme)
	require.NoError(t, err)
	ctx := context.Background()

	tests := []struct {
		name    string
		o       *Firewall
		key     string
		value   string
		wantErr error
		want    *Firewall
	}{
		{
			name: "remove",
			o: &Firewall{
				ObjectMeta: v1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "0",
					Annotations: map[string]string{
						"test": "true",
					},
				},
			},
			key:     "test",
			wantErr: nil,
			want: &Firewall{
				TypeMeta: v1.TypeMeta{
					Kind:       "Firewall",
					APIVersion: "firewall.metal-stack.io/v2",
				},
				ObjectMeta: v1.ObjectMeta{
					Name:            "test",
					ResourceVersion: "1",
					Annotations:     nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.o).Build()

			err := RemoveAnnotation(ctx, c, tt.o, tt.key)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			updated := &Firewall{}
			err = c.Get(ctx, client.ObjectKeyFromObject(tt.o), updated)
			require.NoError(t, err)

			if diff := cmp.Diff(updated, tt.want); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
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
