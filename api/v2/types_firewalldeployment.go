package v2

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// FirewallUserdataCompatibilityAnnotation can be used as an annotation to the firewall deployment resource in order
	// to indicate with which version of the firewall-controller the provided userdata is compatible with.
	//
	// The deployment controller will prevent updates of the managed firewall sets when this constraint violates with
	// the version of the firewall-controller version.
	FirewallUserdataCompatibilityAnnotation = "firewall.metal-stack.io/userdata-firewall-controller-compatibility"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FirewallDeployment contains the spec template of a firewall resource similar to a Kubernetes Deployment and implements update strategies like rolling update for the managed firewalls.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwdeploy
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Progressing",type=integer,JSONPath=`.status.progressingReplicas`
// +kubebuilder:printcolumn:name="Unhealthy",type=integer,JSONPath=`.status.unhealthyReplicas`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type FirewallDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the firewall deployment specification.
	Spec FirewallDeploymentSpec `json:"spec,omitempty"`
	// Status contains current status information on the firewall deployment.
	Status FirewallDeploymentStatus `json:"status,omitempty"`
}

// FirewallUpdateStrategy describes the strategy how firewalls are updated in case the update requires a physical recreation of the firewalls.
type FirewallUpdateStrategy string

const (
	// StrategyRollingUpdate first creates a new firewall set, wait's until it is ready and then removes the old one
	StrategyRollingUpdate FirewallUpdateStrategy = "RollingUpdate"
	// StrategyRecreate removes the old firewall set and then creates a new one
	StrategyRecreate FirewallUpdateStrategy = "Recreate"
)

// FirewallDeploymentSpec specifies the firewall deployment.
type FirewallDeploymentSpec struct {
	// Strategy describes the strategy how firewalls are updated in case the update requires a physical recreation of the firewalls.
	// Defaults to RollingUpdate strategy.
	Strategy FirewallUpdateStrategy `json:"strategy,omitempty"`
	// Replicas is the amount of firewall replicas targeted to be running.
	// Defaults to 1.
	Replicas int `json:"replicas,omitempty"`
	// Selector is a label query over firewalls that should match the replicas count.
	// If selector is empty, it is defaulted to the labels present on the firewall template.
	// Label keys and values that must match in order to be controlled by this replication
	// controller, if empty defaulted to labels on firewall template.
	Selector map[string]string `json:"selector,omitempty"`
	// Template is the firewall spec used for creating the firewalls.
	Template FirewallTemplateSpec `json:"template"`
}

// FirewallDeploymentStatus contains current status information on the firewall deployment.
type FirewallDeploymentStatus struct {
	// TargetReplicas is the amount of firewall replicas targeted to be running.
	TargetReplicas int `json:"targetReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently progressing in the latest managed firewall set.
	ProgressingReplicas int `json:"progressingReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently ready in the latest managed firewall set.
	ReadyReplicas int `json:"readyReplicas"`
	// ProgressingReplicas is the amount of firewall replicas that are currently unhealthy in the latest managed firewall set.
	UnhealthyReplicas int `json:"unhealthyReplicas"`
	// ObservedRevision is a counter that increases with each firewall set roll that was made.
	ObservedRevision int `json:"observedRevision"`
	// Conditions contain the latest available observations of a firewall deployment's current state.
	Conditions Conditions `json:"conditions"`
}

const (
	// FirewallDeplomentAvailable indicates whether the deployment has reached the desired amount of replicas or not.
	FirewallDeplomentAvailable ConditionType = "Available"
	// FirewallDeplomentAvailable indicates whether the deployment has reached the desired amount of replicas or not.
	FirewallDeplomentProgressing ConditionType = "Progressing"
	// FirewallDeplomentRBACProvisioned indicates whether the rbac permissions for the firewall-controller to communicate with the api server were provisioned.
	FirewallDeplomentRBACProvisioned ConditionType = "RBACProvisioned"
)

// FirewallDeploymentList contains a list of firewalls deployments
//
// +kubebuilder:object:root=true
type FirewallDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the list items.
	Items []FirewallDeployment `json:"items"`
}

func (f *FirewallDeploymentList) GetItems() []*FirewallDeployment {
	var result []*FirewallDeployment
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}

func (f *FirewallDeployment) IsFirewallUserdataCompatibilityAnnotationPresent() bool {
	return f.Annotations[FirewallUserdataCompatibilityAnnotation] != ""
}

// IsUserdataCompatibleWithFirewallController returns false if there is a firewall userdata
// annotation present on the firewall deployment, which conflicts with the firewall controller
// version in the firewall spec.
func (f *FirewallDeployment) IsUserdataCompatibleWithFirewallController() (bool, error) {
	rawConstraint := f.Annotations[FirewallUserdataCompatibilityAnnotation]

	constraint, err := semver.NewConstraint(rawConstraint)
	if err != nil {
		return false, fmt.Errorf("firewall-controller userdata constraint is not parsable: %w", err)
	}

	fcv, err := semver.NewVersion(f.Spec.Template.Spec.ControllerVersion)
	if err != nil {
		return false, fmt.Errorf("firewall-controller version is not parsable (maybe an unreleased binary is used?), if you know what you are doing consider removing userdata compatibility annotation manually: %w", err)
	}

	return constraint.Check(fcv), nil
}
