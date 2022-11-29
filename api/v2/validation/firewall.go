package validation

import (
	"net"
	"net/url"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type firewallValidator struct{}

func NewFirewallValidator() *genericValidator[*v2.Firewall, *firewallValidator] {
	return &genericValidator[*v2.Firewall, *firewallValidator]{}
}

func (_ *firewallValidator) New() *firewallValidator {
	return &firewallValidator{}
}

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

func (v *firewallValidator) validateSpecUpdate(fOld, fNew *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(fNew, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.ProjectID, fOld.ProjectID, fldPath.Child("projectID"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.PartitionID, fOld.PartitionID, fldPath.Child("partitionID"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.SSHPublicKeys, fOld.SSHPublicKeys, fldPath.Child("sshPublicKeys"))...)

	return allErrs
}

func (_ *firewallValidator) validateSpec(f *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	r := requiredFields{
		{path: fldPath.Child("controllerURL"), value: f.ControllerURL},
		{path: fldPath.Child("controllerVersion"), value: f.ControllerVersion},
		{path: fldPath.Child("image"), value: f.Image},
		{path: fldPath.Child("partitionID"), value: f.PartitionID},
		{path: fldPath.Child("projectID"), value: f.ProjectID},
		{path: fldPath.Child("size"), value: f.Size},
		{path: fldPath.Child("networks"), value: f.Networks},
	}

	allErrs = append(allErrs, r.check()...)

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

	for _, rule := range f.EgressRules {
		rule := rule

		r = requiredFields{
			{path: fldPath.Child("egressRules").Child("networkID"), value: rule.NetworkID},
		}
		allErrs = append(allErrs, r.check()...)

		for _, ip := range rule.IPs {
			if parsed := net.ParseIP(ip); parsed == nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("egressRules").Child("ips"), ip, "ip must be a parsable ip adddress"))
			}
		}
	}

	for _, prefix := range f.InternalPrefixes {
		if _, _, err := net.ParseCIDR(prefix); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("internalPrefixes"), prefix, "prefix must be a parsable network cidr"))
		}
	}

	for _, limit := range f.RateLimits {
		limit := limit

		r = requiredFields{
			{path: fldPath.Child("rateLimits").Child("networkID"), value: limit.NetworkID},
		}
		allErrs = append(allErrs, r.check()...)
	}

	return allErrs
}
