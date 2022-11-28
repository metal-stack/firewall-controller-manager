package controllers

import (
	"fmt"
	"strconv"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// most of the stuff copied/inspired from/by k8s deployment controller util

const (
	// RevisionAnnotation is the revision annotation of a deployment's replica sets which records its rollout sequence
	RevisionAnnotation = "deployment.kubernetes.io/revision"
)

// Revision returns the revision number of the input object.
func Revision(obj runtime.Object) (int64, error) {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return 0, err
	}
	v, ok := acc.GetAnnotations()[RevisionAnnotation]
	if !ok {
		return 0, nil
	}
	return strconv.ParseInt(v, 10, 64)
}

// MaxRevisionOf finds the highest revision in the firewall sets
func MaxRevisionOf(sets []*v2.FirewallSet) (*v2.FirewallSet, error) {
	var result *v2.FirewallSet
	max := int64(0)
	for _, set := range sets {
		set := set
		if result == nil {
			result = set
		}
		if v, err := Revision(set); err != nil {
			return nil, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
		} else if v > max {
			max = v
			result = set
		}
	}
	return result, nil
}

func Except(sets []*v2.FirewallSet, set *v2.FirewallSet) []*v2.FirewallSet {
	var result []*v2.FirewallSet
	for _, s := range sets {
		s := s
		if set.UID == s.UID {
			continue
		}
		result = append(result, s)
	}
	return result
}
