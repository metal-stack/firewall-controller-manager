package validation

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type firewallSetValidator struct{}

func NewFirewallSetValidator() *genericValidator[*v2.FirewallSet, *firewallSetValidator] {
	return &genericValidator[*v2.FirewallSet, *firewallSetValidator]{}
}

func (v *firewallSetValidator) ValidateCreate(f *v2.FirewallSet) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(&f.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, NewFirewallValidator().Instance().validateSpec(&f.Spec.Template, field.NewPath("spec").Child("template"))...)

	return allErrs
}

func (v *firewallSetValidator) ValidateUpdate(oldF, newF *v2.FirewallSet) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpecUpdate(&oldF.Spec, &newF.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Userdata, oldF.Userdata, field.NewPath("userdata"))...)
	allErrs = append(allErrs, NewFirewallValidator().Instance().validateSpecUpdate(&oldF.Spec.Template, &newF.Spec.Template, field.NewPath("spec").Child("template"))...)

	return allErrs
}

func (v *firewallSetValidator) validateSpecUpdate(_, fNew *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(fNew, fldPath)...)

	return allErrs
}

func (_ *firewallSetValidator) validateSpec(f *v2.FirewallSetSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if f.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "replicas cannot be a negative number"))
	}
	if f.Replicas > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "for now, no more than a single firewall replica is allowed"))
	}

	return allErrs
}
