package validation

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

type (
	requiredFields []*requiredField
	requiredField  struct {
		value any
		path  *field.Path
	}
)

func (rs requiredFields) check() field.ErrorList {
	var allErrs field.ErrorList

	for _, r := range rs {
		if reflect.ValueOf(r.value).IsZero() {
			allErrs = append(allErrs, field.Required(r.path, "field is required"))
		}
	}

	return allErrs
}
