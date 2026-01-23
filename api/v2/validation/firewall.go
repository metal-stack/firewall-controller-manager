package validation

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type firewallValidator struct {
	log logr.Logger
}

func NewFirewallValidator(log logr.Logger) admission.Validator[*v2.Firewall] {
	return &firewallValidator{
		log: log,
	}
}

func (v *firewallValidator) ValidateCreate(ctx context.Context, f *v2.Firewall) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&f.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateFirewallAnnotations(f)...)
	allErrs = append(allErrs, validateFirewallSpec(&f.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, validateDistance(f.Distance, field.NewPath("distance"))...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		f.GetObjectKind().GroupVersionKind().GroupKind(),
		f.GetName(),
		allErrs,
	)
}

func (v *firewallValidator) ValidateUpdate(ctx context.Context, fOld, fNew *v2.Firewall) (admission.Warnings, error) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaAccessorUpdate(&fNew.ObjectMeta, &fOld.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateFirewallAnnotations(fNew)...)
	allErrs = append(allErrs, validateFirewallSpecUpdate(&fOld.Spec, &fNew.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, validateDistance(fNew.Distance, field.NewPath("distance"))...)

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		fNew.GetObjectKind().GroupVersionKind().GroupKind(),
		fNew.GetName(),
		allErrs,
	)
}

func (v *firewallValidator) ValidateDelete(ctx context.Context, f *v2.Firewall) (admission.Warnings, error) {
	return nil, nil
}

func validateFirewallSpec(f *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	r := requiredFields{
		{path: fldPath.Child("controllerURL"), value: f.ControllerURL},
		{path: fldPath.Child("controllerVersion"), value: f.ControllerVersion},
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

	if f.NftablesExporterURL != "" {
		_, err = url.ParseRequestURI(f.NftablesExporterURL)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("nftablesExporterURL"), f.NftablesExporterURL, "url must be parsable http url"))
		}
	}

	for _, rule := range f.EgressRules {
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
		r = requiredFields{
			{path: fldPath.Child("rateLimits").Child("networkID"), value: limit.NetworkID},
		}
		allErrs = append(allErrs, r.check()...)
	}

	for _, cidr := range f.AllowedNetworks.Egress {
		_, err := netip.ParsePrefix(cidr)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("allowedNetworks").Child("egress"), cidr, fmt.Sprintf("given network must be a cidr: %v", err)))
		}
	}

	for _, cidr := range f.AllowedNetworks.Ingress {
		_, err := netip.ParsePrefix(cidr)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("allowedNetworks").Child("ingress"), cidr, fmt.Sprintf("given network must be a cidr: %v", err)))
		}
	}

	return allErrs
}

func validateFirewallSpecUpdate(fOld, fNew *v2.FirewallSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateFirewallSpec(fNew, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.Project, fOld.Project, fldPath.Child("project"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.Partition, fOld.Partition, fldPath.Child("partition"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(fNew.DNSPort, fOld.DNSPort, fldPath.Child("dnsPort"))...)

	return allErrs
}

func validateFirewallAnnotations(f *v2.Firewall) field.ErrorList {
	var allErrs field.ErrorList

	if v, ok := f.Annotations[v2.FirewallNoControllerConnectionAnnotation]; ok {
		_, err := strconv.ParseBool(v)
		if err != nil {
			allErrs = append(allErrs, field.TypeInvalid(field.NewPath("metadata").Child("annotations"), v, fmt.Sprintf("value of %q must be parsable as bool", v2.FirewallNoControllerConnectionAnnotation)))
		}
	}

	if v, ok := f.Annotations[v2.FirewallWeightAnnotation]; ok {
		_, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			allErrs = append(allErrs, field.TypeInvalid(field.NewPath("metadata").Child("annotations"), v, fmt.Sprintf("value of %q must be parsable as int", v2.FirewallWeightAnnotation)))
		}
	}

	return allErrs
}

func validateDistance(distance v2.FirewallDistance, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if distance < v2.FirewallShortestDistance || distance > v2.FirewallLongestDistance {
		allErrs = append(allErrs, field.Invalid(fldPath, distance, fmt.Sprintf("distance must be between %d and %d", v2.FirewallShortestDistance, v2.FirewallLongestDistance)))
	}

	return allErrs
}
