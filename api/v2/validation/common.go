package validation

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type genericValidation[O client.Object] interface {
	ValidateCreate(log logr.Logger, obj O) field.ErrorList
	ValidateUpdate(log logr.Logger, old, new O) field.ErrorList
}

type genericValidator[O client.Object, V genericValidation[O]] struct {
	log logr.Logger
}

func (g *genericValidator[O, V]) Instance() V {
	var v V
	return v
}

func (g *genericValidator[O, V]) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	var (
		v       = g.Instance()
		o, ok   = obj.(O)
		allErrs field.ErrorList
	)

	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", obj))
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}

	g.log.Info("validating resource creation", "name", accessor.GetName(), "namespace", accessor.GetNamespace())

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessor(accessor, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.ValidateCreate(g.log, o)...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		obj.GetObjectKind().GroupVersionKind().GroupKind(),
		accessor.GetName(),
		allErrs,
	)
}

func (g *genericValidator[O, V]) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var (
		v           = g.Instance()
		oldO, oldOk = oldObj.(O)
		newO, newOk = newObj.(O)
		allErrs     field.ErrorList
	)

	if !oldOk {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", oldO))
	}
	if !newOk {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("validator received unexpected type: %T", newO))
	}

	oldAccessor, err := meta.Accessor(oldO)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}
	newAccessor, err := meta.Accessor(newO)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to get accessor for object: %s", err))
	}

	g.log.Info("validating resource update", "name", newAccessor.GetName(), "namespace", newAccessor.GetNamespace())

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessorUpdate(newAccessor, oldAccessor, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.ValidateUpdate(g.log, oldO, newO)...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		newO.GetObjectKind().GroupVersionKind().GroupKind(),
		newAccessor.GetName(),
		allErrs,
	)
}

func (_ *genericValidator[O, V]) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

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
		r := r

		if reflect.ValueOf(r.value).IsZero() {
			allErrs = append(allErrs, field.Required(r.path, "field is required"))
		}
	}

	return allErrs
}
