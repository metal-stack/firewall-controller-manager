package validation

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type firewallSetValidator struct {
	log logr.Logger
}

func NewFirewallSetValidator(log logr.Logger) admission.Validator[*v2.FirewallSet] {
	return &firewallSetValidator{
		log: log,
	}
}

func (v *firewallSetValidator) ValidateCreate(ctx context.Context, f *v2.FirewallSet) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessor(&f.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.validateSpec(v.log, &f.Spec, field.NewPath("spec"))...)

	return nil, apierrors.NewInvalid(
		f.GetObjectKind().GroupVersionKind().GroupKind(),
		f.GetName(),
		allErrs,
	)
}

func (v *firewallSetValidator) ValidateUpdate(ctx context.Context, oldF, newF *v2.FirewallSet) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessorUpdate(&newF.ObjectMeta, &oldF.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.validateSpecUpdate(v.log, &oldF.Spec, &newF.Spec, field.NewPath("spec"))...)

	return nil, apierrors.NewInvalid(
		newF.GetObjectKind().GroupVersionKind().GroupKind(),
		newF.GetName(),
		allErrs,
	)
}

func (v *firewallSetValidator) ValidateDelete(ctx context.Context, f *v2.FirewallSet) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (v *firewallSetValidator) validateSpec(log logr.Logger, f *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if f.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "replicas cannot be a negative number"))
	}
	if f.Replicas > v2.FirewallMaxReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, fmt.Sprintf("no more than %d firewall replicas are allowed", v2.FirewallMaxReplicas)))
	}

	allErrs = append(allErrs, validateDistance(f.Distance, fldPath.Child("distance"))...)

	if f.Selector == nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), f.Selector, "selector should not be nil"))
	} else {
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: f.Selector,
		})
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), f.Selector, ""))
		}

		if !selector.Empty() {
			labels := labels.Set(f.Template.Labels)
			if !selector.Matches(labels) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("template", "metadata", "labels"), f.Template.Labels, "`selector` does not match template `labels`"))
			}
		}
	}

	allErrs = append(allErrs, validateFirewallSpec(&f.Template.Spec, fldPath.Child("template").Child("spec"))...)

	return allErrs
}

func (v *firewallSetValidator) validateSpecUpdate(log logr.Logger, oldF, newF *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(log, newF, fldPath)...)

	allErrs = append(allErrs, validateFirewallSpecUpdate(&oldF.Template.Spec, &newF.Template.Spec, fldPath.Child("template").Child("spec"))...)

	// TODO: theoretically, the selector or metadata should be changeable, but we need to think it through... let's simplify for now and just not support it.
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Selector, oldF.Selector, fldPath.Child("selector"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Template.ObjectMeta, oldF.Template.ObjectMeta, fldPath.Child("template").Child("metadata"))...)

	return allErrs
}
