package controllers

import (
	"fmt"
	"reflect"
	"strconv"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// most of the stuff inspired from/by k8s deployment controller util

const (
	RevisionAnnotation = "firewall-deployment.metal-stack.io/revision"
)

// Revision returns the revision number of the input object.
func Revision(obj runtime.Object) (int, error) {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return 0, err
	}
	v, ok := acc.GetAnnotations()[RevisionAnnotation]
	if !ok {
		return 0, nil
	}
	return strconv.Atoi(v)
}

// MaxRevisionOf finds the highest revision in the firewall sets
func MaxRevisionOf(sets []*v2.FirewallSet) (*v2.FirewallSet, error) {
	var (
		max    int
		result *v2.FirewallSet
	)

	if len(sets) == 0 {
		return result, nil
	} else {
		set := sets[0]
		v, err := Revision(set)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
		}
		max = v
		result = set
	}

	for _, set := range sets {
		set := set

		if v, err := Revision(set); err != nil {
			return nil, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
		} else if v > max {
			max = v
			result = set
		}
	}

	return result, nil
}

// MinRevisionOf finds the lowest revision in the firewall sets
func MinRevisionOf(sets []*v2.FirewallSet) (*v2.FirewallSet, error) {
	var (
		min    int
		result *v2.FirewallSet
	)

	if len(sets) == 0 {
		return result, nil
	} else {
		set := sets[0]
		v, err := Revision(set)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
		}
		min = v
		result = set
	}

	for _, set := range sets {
		set := set

		if v, err := Revision(set); err != nil {
			return nil, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
		} else if v < min {
			min = v
			result = set
		}
	}

	return result, nil
}

func Except[O client.Object](sets []O, except ...O) []O {
	var result []O

	for _, s := range sets {
		s := s

		if reflect.ValueOf(s).IsNil() {
			continue
		}

		found := false
		for _, e := range except {
			e := e

			if reflect.ValueOf(e).IsNil() {
				continue
			}

			if e.GetUID() == s.GetUID() {
				found = true
				break
			}
		}

		if found {
			continue
		}

		result = append(result, s)
	}

	return result
}

func NextRevision(o runtime.Object) (int, error) {
	i, err := Revision(o)
	if err != nil {
		return 0, fmt.Errorf("couldn't parse revision for firewall set: %w", err)
	}

	i++

	return i, nil
}
