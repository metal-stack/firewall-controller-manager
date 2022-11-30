package validation

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type firewallDeploymentValidator struct{}

func NewFirewallDeploymentValidator() *genericValidator[*v2.FirewallDeployment, *firewallDeploymentValidator] {
	return &genericValidator[*v2.FirewallDeployment, *firewallDeploymentValidator]{}
}

func (v *firewallDeploymentValidator) ValidateCreate(f *v2.FirewallDeployment) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(&f.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, NewFirewallValidator().Instance().validateSpec(&f.Spec.Template, field.NewPath("spec").Child("template"))...)

	return allErrs
}

func (v *firewallDeploymentValidator) ValidateUpdate(oldF, newF *v2.FirewallDeployment) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpecUpdate(&oldF.Spec, &newF.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, NewFirewallValidator().Instance().validateSpecUpdate(&oldF.Spec.Template, &newF.Spec.Template, field.NewPath("spec").Child("template"))...)

	return allErrs
}

func (v *firewallDeploymentValidator) validateSpecUpdate(fOld, fNew *v2.FirewallDeploymentSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(fNew, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.Strategy, fOld.Strategy, fldPath.Child("strategy"))...)

	return allErrs
}

func (*firewallDeploymentValidator) validateSpec(f *v2.FirewallDeploymentSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if f.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "replicas cannot be a negative number"))
	}
	if f.Replicas > 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "for now, no more than a single firewall replica is allowed"))
	}
	if f.Strategy != v2.StrategyRecreate && f.Strategy != v2.StrategyRollingUpdate {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("strategy"), f.Strategy, fmt.Sprintf("unknown strategy: %s", f.Strategy)))
	}

	return allErrs
}
