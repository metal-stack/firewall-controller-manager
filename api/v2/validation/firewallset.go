package validation

import (
	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type firewallSetValidator struct{}

func NewFirewallSetValidator(log logr.Logger) *genericValidator[*v2.FirewallSet, *firewallSetValidator] {
	return &genericValidator[*v2.FirewallSet, *firewallSetValidator]{log: log}
}

func (v *firewallSetValidator) ValidateCreate(log logr.Logger, f *v2.FirewallSet) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(log, &f.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (v *firewallSetValidator) validateSpec(log logr.Logger, f *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if f.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "replicas cannot be a negative number"))
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

	allErrs = append(allErrs, NewFirewallValidator(log).Instance().validateSpec(&f.Template.Spec, fldPath.Child("template").Child("spec"))...)

	return allErrs
}

func (v *firewallSetValidator) ValidateUpdate(log logr.Logger, oldF, newF *v2.FirewallSet) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpecUpdate(log, &oldF.Spec, &newF.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (v *firewallSetValidator) validateSpecUpdate(log logr.Logger, oldF, newF *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(log, newF, fldPath)...)

	allErrs = append(allErrs, NewFirewallValidator(log).Instance().validateSpecUpdate(&oldF.Template.Spec, &newF.Template.Spec, fldPath.Child("template").Child("spec"))...)

	// TODO: theoretically, the selector or metadata should be changeable, but we need to think it through... let's simplify for now and just not support it.
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Selector, oldF.Selector, fldPath.Child("selector"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Template.ObjectMeta, oldF.Template.ObjectMeta, fldPath.Child("template").Child("metadata"))...)

	return allErrs
}
