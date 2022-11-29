package validation

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewFirewallDeploymentValidator() admission.CustomValidator {
	return &genericValidator[*v2.FirewallDeployment]{v: &firewallDeploymentValidator{}}
}

type firewallDeploymentValidator struct{}

func (v *firewallDeploymentValidator) ValidateCreate(f *v2.FirewallDeployment) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.commonFieldValidation(f)...)

	return allErrs
}

func (v *firewallDeploymentValidator) ValidateUpdate(oldF, newF *v2.FirewallDeployment) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.commonFieldValidation(newF)...)

	return allErrs
}

func (_ *firewallDeploymentValidator) commonFieldValidation(f *v2.FirewallDeployment) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&f.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	if f.Spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.replicas"), f.Spec.Replicas, "replicas cannot be a negative number"))
	}
	if f.Spec.Replicas > 1 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.replicas"), f.Spec.Replicas, "for now, no more than a single firewall replica is allowed"))
	}
	if f.Spec.Strategy != v2.StrategyRecreate && f.Spec.Strategy != v2.StrategyRollingUpdate {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.strategy"), f.Spec.Strategy, fmt.Sprintf("unknown strategy: %s", f.Spec.Strategy)))
	}

	return allErrs
}
