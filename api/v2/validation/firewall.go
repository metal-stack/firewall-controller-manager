package validation

import (
	"net/url"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewFirewallValidator() admission.CustomValidator {
	return &genericValidator[*v2.Firewall]{v: &firewallValidator{}}
}

type firewallValidator struct{}

func (v *firewallValidator) ValidateCreate(f *v2.Firewall) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(&f.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (v *firewallValidator) ValidateUpdate(oldF, newF *v2.Firewall) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpecUpdate(&oldF.Spec, &newF.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (_ *firewallValidator) validateSpecUpdate(fOld *v2.FirewallSpec, fNew *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.ProjectID, fOld.ProjectID, fldPath.Child("projectID"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.PartitionID, fOld.PartitionID, fldPath.Child("partitionID"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.SSHPublicKeys, fOld.SSHPublicKeys, fldPath.Child("sshpublickeys"))...)

	return allErrs
}

func (_ *firewallValidator) validateSpec(f *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	r := requiredFields{
		{path: "controllerURL", value: f.ControllerURL},
		{path: "controllerVersion", value: f.ControllerVersion},
		{path: "image", value: f.Image},
		{path: "partitionID", value: f.PartitionID},
		{path: "projectID", value: f.ProjectID},
		{path: "size", value: f.Size},
		{path: "networks", value: f.Networks},
	}

	allErrs = append(allErrs, r.check(fldPath)...)

	d, err := time.ParseDuration(f.Interval)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("interval"), f.Interval, "interval must be parsable as a duration"))
	} else {
		if d == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("interval"), f.Interval, "interval must be larger than 0"))
		}
	}

	_, err = url.ParseRequestURI(f.ControllerURL)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("controllerURL"), f.ControllerURL, "url must be parsable http url"))
	}

	return allErrs
}
