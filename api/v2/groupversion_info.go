// +kubebuilder:object:generate=true
// +groupName=metal-stack.io
//
// +kubebuilder:webhook:path=/validate-metal-stack-io-v2-firewall,mutating=false,failurePolicy=fail,groups=metal-stack.io,resources=firewalls,verbs=create;update,versions=v2,name=firewall.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-metal-stack-io-v2-firewallset,mutating=false,failurePolicy=fail,groups=metal-stack.io,resources=firewallsets,verbs=create;update,versions=v2,name=firewallset.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-metal-stack-io-v2-firewalldeployment,mutating=false,failurePolicy=fail,groups=metal-stack.io,resources=firewalldeployments,verbs=create;update,versions=v2,name=firewalldeployment.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
//
// +kubebuilder:webhook:path=/mutate-metal-stack-io-v2-firewall,mutating=true,failurePolicy=fail,groups=metal-stack.io,resources=firewalls,verbs=create;update,versions=v2,name=firewall.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/mutate-metal-stack-io-v2-firewallset,mutating=true,failurePolicy=fail,groups=metal-stack.io,resources=firewallsets,verbs=create;update,versions=v2,name=firewallset.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/mutate-metal-stack-io-v2-firewalldeployment,mutating=true,failurePolicy=fail,groups=metal-stack.io,resources=firewalldeployments,verbs=create;update,versions=v2,name=firewalldeployment.metal-stack.io,sideEffects=None,admissionReviewVersions=v1
package v2

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "metal-stack.io", Version: "v2"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&Firewall{},
		&FirewallList{},
		&FirewallSet{},
		&FirewallSetList{},
		&FirewallDeployment{},
		&FirewallDeploymentList{},
		&FirewallMonitor{},
		&FirewallMonitorList{},
	)
}
