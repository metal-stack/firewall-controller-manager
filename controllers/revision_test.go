package controllers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMaxRevisionOf(t *testing.T) {
	tests := []struct {
		name    string
		sets    []*v2.FirewallSet
		want    *v2.FirewallSet
		wantErr error
	}{
		{
			name: "empty",
			sets: []*v2.FirewallSet{},
			want: nil,
		},
		{
			name: "single revision",
			sets: []*v2.FirewallSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "a",
						Annotations: map[string]string{
							RevisionAnnotation: "0",
						},
					},
				},
			},
			want: &v2.FirewallSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "a",
					Annotations: map[string]string{
						RevisionAnnotation: "0",
					},
				},
			},
		},
		{
			name: "max in the middle",
			sets: []*v2.FirewallSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "a",
						Annotations: map[string]string{
							RevisionAnnotation: "0",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "c",
						Annotations: map[string]string{
							RevisionAnnotation: "2",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "b",
						Annotations: map[string]string{
							RevisionAnnotation: "1",
						},
					},
				},
			},
			want: &v2.FirewallSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "c",
					Annotations: map[string]string{
						RevisionAnnotation: "2",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := MaxRevisionOf(tt.sets)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}

func TestMinRevisionOf(t *testing.T) {
	tests := []struct {
		name    string
		sets    []*v2.FirewallSet
		want    *v2.FirewallSet
		wantErr error
	}{
		{
			name: "empty",
			sets: []*v2.FirewallSet{},
			want: nil,
		},
		{
			name: "single revision",
			sets: []*v2.FirewallSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "a",
						Annotations: map[string]string{
							RevisionAnnotation: "0",
						},
					},
				},
			},
			want: &v2.FirewallSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "a",
					Annotations: map[string]string{
						RevisionAnnotation: "0",
					},
				},
			},
		},
		{
			name: "min in the middle",
			sets: []*v2.FirewallSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "c",
						Annotations: map[string]string{
							RevisionAnnotation: "2",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "a",
						Annotations: map[string]string{
							RevisionAnnotation: "0",
						},
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "b",
						Annotations: map[string]string{
							RevisionAnnotation: "1",
						},
					},
				},
			},
			want: &v2.FirewallSet{
				ObjectMeta: v1.ObjectMeta{
					Name: "a",
					Annotations: map[string]string{
						RevisionAnnotation: "0",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := MinRevisionOf(tt.sets)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}

func TestExcept(t *testing.T) {
	tests := []struct {
		name   string
		sets   []*v2.FirewallSet
		except []*v2.FirewallSet
		want   []*v2.FirewallSet
	}{
		{
			name: "exlclude middle",
			sets: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
			},
			except: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
			},
			want: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
			},
		},
		{
			name: "exlclude two resources",
			sets: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("4")}},
			},
			except: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("4")}},
			},
			want: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
			},
		},
		{
			name: "allow nil",
			sets: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				nil,
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
			},
			except: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("2")}},
				nil,
			},
			want: []*v2.FirewallSet{
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("1")}},
				{ObjectMeta: v1.ObjectMeta{UID: types.UID("3")}},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := Except(tt.sets, tt.except...)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
