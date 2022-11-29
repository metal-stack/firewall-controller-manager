package validation

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericValidation[O client.Object] interface {
	ValidateCreate(O) field.ErrorList
	ValidateUpdate(old, new O) field.ErrorList
}

type genericValidator[O client.Object] struct {
	v genericValidation[O]
}

func (g *genericValidator[O]) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	var (
		o, ok   = obj.(O)
		allErrs field.ErrorList
	)

	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", obj))
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessor(accessor, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, g.v.ValidateCreate(o)...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		obj.GetObjectKind().GroupVersionKind().GroupKind(),
		accessor.GetName(),
		allErrs,
	)
}

func (g *genericValidator[O]) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	var (
		oldO, oldOk = oldObj.(O)
		newO, newOk = newObj.(O)
		allErrs     field.ErrorList
	)

	if !oldOk {
		return apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", oldO))
	}
	if !newOk {
		return apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", newO))
	}

	oldAccessor, err := meta.Accessor(oldO)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}
	newAccessor, err := meta.Accessor(newO)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessorUpdate(newAccessor, oldAccessor, field.NewPath("metadata"))...)
	allErrs = append(allErrs, g.v.ValidateUpdate(oldO, newO)...)

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		newO.GetObjectKind().GroupVersionKind().GroupKind(),
		newAccessor.GetName(),
		allErrs,
	)
}

func (_ *genericValidator[O]) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

type (
	requiredFields []*requiredField
	requiredField  struct {
		value any
		path  string
	}
)

func (rs requiredFields) check(fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	for _, r := range rs {
		r := r

		if reflect.ValueOf(r.value).IsZero() {
			allErrs = append(allErrs, field.Required(fldPath.Child(r.path), fmt.Sprintf("%s is required", r.path)))
		}
	}

	return allErrs
}
