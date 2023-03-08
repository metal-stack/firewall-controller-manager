package validation

import (
	"fmt"
	"net/netip"
	"net/url"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type firewallValidator struct{}

func NewFirewallValidator(log logr.Logger) *genericValidator[*v2.Firewall, *firewallValidator] {
	return &genericValidator[*v2.Firewall, *firewallValidator]{log: log}
}

func (v *firewallValidator) ValidateCreate(log logr.Logger, f *v2.Firewall) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(&f.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (*firewallValidator) validateSpec(f *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	r := requiredFields{
		{path: fldPath.Child("controllerURL"), value: f.ControllerURL},
		{path: fldPath.Child("controllerVersion"), value: f.ControllerVersion},
		{path: fldPath.Child("nftablesExporterURL"), value: f.NftablesExporterURL},
		{path: fldPath.Child("nftablesExporterVersion"), value: f.NftablesExporterVersion},
		{path: fldPath.Child("image"), value: f.Image},
		{path: fldPath.Child("partition"), value: f.Partition},
		{path: fldPath.Child("project"), value: f.Project},
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

	_, err = url.ParseRequestURI(f.NftablesExporterURL)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("nftablesExporterURL"), f.NftablesExporterURL, "url must be parsable http url"))
	}

	for _, rule := range f.EgressRules {
		rule := rule

		r = requiredFields{
			{path: fldPath.Child("egressRules").Child("networkID"), value: rule.NetworkID},
		}
		allErrs = append(allErrs, r.check()...)

		for _, ip := range rule.IPs {
			if _, err := netip.ParseAddr(ip); err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("egressRules").Child("ips"), ip, fmt.Sprintf("error parsing ip: %v", err)))
			}
		}
	}

	for _, prefix := range f.InternalPrefixes {
		if _, err := netip.ParsePrefix(prefix); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("internalPrefixes"), prefix, fmt.Sprintf("error parsing prefix: %v", err)))
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

func (v *firewallValidator) ValidateUpdate(log logr.Logger, fOld, fNew *v2.Firewall) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpecUpdate(&fOld.Spec, &fNew.Spec, field.NewPath("spec"))...)

	return allErrs
}

func (v *firewallValidator) validateSpecUpdate(fOld, fNew *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, v.validateSpec(fNew, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.Project, fOld.Project, fldPath.Child("project"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.Partition, fOld.Partition, fldPath.Child("partition"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.DNSPort, fOld.DNSPort, fldPath.Child("dnsPort"))...)

	return allErrs
}
