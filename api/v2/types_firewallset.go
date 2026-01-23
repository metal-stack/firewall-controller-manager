package v2

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	FirewallShortestDistance         = FirewallDistance(0)
	FirewallRollingUpdateSetDistance = FirewallDistance(3)
	FirewallLongestDistance          = FirewallDistance(8)

	// FirewallMaxReplicas defines the maximum amount of firewall replicas to be defined.
	// It does not make sense to allow large values here as it wastes a lot of machines.
	FirewallMaxReplicas = 4
)

func FirewallSetTag(setName string) string {
	return fmt.Sprintf("%s=%s", FirewallControllerSetAnnotation, setName)
}

func FirewallManagedByTag() string {
	return fmt.Sprintf("%s=%s", FirewallControllerManagedByAnnotation, FirewallControllerManager)
}

func (f FirewallDistance) Pointer() *FirewallDistance {
	return &f
}

// FirewallSet contains the spec template of a firewall resource similar to a Kubernetes ReplicaSet and takes care that the desired amount of firewall replicas is running.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwset
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.readyReplicas
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyReplicas`
// +kubebuilder:printcolumn:name="Progressing",type=integer,JSONPath=`.status.progressingReplicas`
// +kubebuilder:printcolumn:name="Unhealthy",type=integer,JSONPath=`.status.unhealthyReplicas`
// +kubebuilder:printcolumn:name="Distance",type="string",priority=1,JSONPath=".spec.distance"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type FirewallSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec contains the firewall set specification.
	Spec FirewallSetSpec `json:"spec"`
	// Status contains current status information on the firewall set.
	Status FirewallSetStatus `json:"status"`
}

// FirewallSetSpec specifies the firewall set.
type FirewallSetSpec struct {
	// Replicas is the amount of firewall replicas targeted to be running.
	Replicas int `json:"replicas"`
	// Selector is a label query over firewalls that should match the replicas count.
	// If selector is empty, it is defaulted to the labels present on the firewall template.
	// Label keys and values that must match in order to be controlled by this replication
	// controller, if empty defaulted to labels on firewall template.
	Selector map[string]string `json:"selector,omitempty"`
	// Template is the firewall spec used for creating the firewalls.
	Template FirewallTemplateSpec `json:"template"`
	// Distance defines the as-path length of the firewalls.
	// This field is typically orchestrated by the deployment controller.
	Distance FirewallDistance `json:"distance"`
}

// FirewallDistance defines the as-path length of firewalls, influencing how strong they attract
// network traffic for routing traffic in and out of the cluster.
// This is of particular interest during rolling firewall updates, i.e. when there is
// more than a single firewall running in front of the cluster.
// During a rolling update, new firewalls start with a longer distance such that
// traffic is only attracted by the existing firewalls ("firewall staging").
// When the new firewall has connected successfully to the firewall monitor, the deployment
// controller throws away the old firewalls and the new firewall takes over the routing.
// The deployment controller will then shorten the distance of the new firewall.
// This approach reduces service interruption of the external user traffic of the cluster
// (for firewall-controller versions that support this feature).
type FirewallDistance uint8

type FirewallSetStatus struct {
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
}

// FirewallSetList contains a list of firewalls sets
//
// +kubebuilder:object:root=true
type FirewallSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items contains the list items.
	Items []FirewallSet `json:"items"`
}

func (f *FirewallSetList) GetItems() []*FirewallSet {
	var result []*FirewallSet
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
