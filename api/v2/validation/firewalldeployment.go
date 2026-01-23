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

type firewallDeploymentValidator struct {
	log logr.Logger
}

func NewFirewallDeploymentValidator(log logr.Logger) admission.Validator[*v2.FirewallDeployment] {
	return &firewallDeploymentValidator{
		log: log,
	}
}

func (v *firewallDeploymentValidator) ValidateCreate(ctx context.Context, f *v2.FirewallDeployment) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessor(&f.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.validateSpec(&f.Spec, field.NewPath("spec"))...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		f.GetObjectKind().GroupVersionKind().GroupKind(),
		f.GetName(),
		allErrs,
	)
}

func (v *firewallDeploymentValidator) ValidateUpdate(ctx context.Context, oldF, newF *v2.FirewallDeployment) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessorUpdate(&newF.ObjectMeta, &oldF.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, v.validateSpecUpdate(v.log, &oldF.Spec, &newF.Spec, &newF.Status, field.NewPath("spec"))...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		newF.GetObjectKind().GroupVersionKind().GroupKind(),
		newF.GetName(),
		allErrs,
	)
}

func (v *firewallDeploymentValidator) ValidateDelete(ctx context.Context, f *v2.FirewallDeployment) (warnings admission.Warnings, err error) {
	return nil, nil
}

func (*firewallDeploymentValidator) validateSpec(f *v2.FirewallDeploymentSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	switch f.Strategy {
	case v2.StrategyRecreate, v2.StrategyRollingUpdate:
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("strategy"), f.Strategy, fmt.Sprintf("unknown strategy: %s", f.Strategy)))
	}

	if f.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, "replicas cannot be a negative number"))
	}
	if f.Replicas > v2.FirewallMaxReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), f.Replicas, fmt.Sprintf("no more than %d firewall replicas are allowed", v2.FirewallMaxReplicas)))
	}

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

func (v *firewallDeploymentValidator) validateSpecUpdate(log logr.Logger, oldF, newF *v2.FirewallDeploymentSpec, status *v2.FirewallDeploymentStatus, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(newF, fldPath)...)

	allErrs = append(allErrs, validateFirewallSpecUpdate(&oldF.Template.Spec, &newF.Template.Spec, fldPath.Child("template").Child("spec"))...)

	// TODO: theoretically, the selector or metadata should be changeable, but we need to think it through... let's simplify for now and just not support it.
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Selector, oldF.Selector, fldPath.Child("selector"))...)

	if newF.Strategy != oldF.Strategy && status.TargetReplicas != status.ReadyReplicas {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("strategy"), newF.Strategy, "strategy can not be updated until target replicas have been reached (i.e. deployment has converged)"))
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newF.Template.ObjectMeta, oldF.Template.ObjectMeta, fldPath.Child("template").Child("metadata"))...)

	return allErrs
}
