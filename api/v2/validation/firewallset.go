package validation

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewFirewallSetValidator() admission.CustomValidator {
	return &genericValidator[*v2.FirewallSet]{v: &firewallSetValidator{}}
}

type firewallSetValidator struct{}

func (v *firewallSetValidator) ValidateCreate(f *v2.FirewallSet) field.ErrorList {
	return nil
}

func (v *firewallSetValidator) ValidateUpdate(oldF, newF *v2.FirewallSet) field.ErrorList {
	return nil
}
