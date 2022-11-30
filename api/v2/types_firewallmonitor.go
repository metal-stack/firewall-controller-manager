package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwmon
// +kubebuilder:printcolumn:name="Machine ID",type="string",JSONPath=".machineStatus.machineID"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".image"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".size"
// +kubebuilder:printcolumn:name="Last Event",type="string",JSONPath=".machineStatus.lastEvent.event"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".machineStatus.allocationTimestamp"
type FirewallMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Size is the size of the firewall
	Size string `json:"size"`
	// Image is the os image installed on the firewall
	Image string `json:"image"`
	// PartitionID is the partition in which the firewall is running
	PartitionID string `json:"partitionID"`
	// ProjectID is the project for which the firewall was created
	ProjectID string `json:"projectID"`
	// Networks are the networks to which this firewall is connected
	Networks []string `json:"networks"`
	// RateLimits contains the configuration of rate limit rules for interfaces
	RateLimits []RateLimit `json:"rateLimits,omitempty"`
	// EgressRules contains egress rules configured for this firewall
	EgressRules []EgressRuleSNAT `json:"egressRules,omitempty"`
	// LogAcceptedConnections if set to true, also log accepted connections in the droptailer log
	LogAcceptedConnections bool `json:"logAcceptedConnections,omitempty"`
	// MachineStatus holds the status of the firewall machine
	MachineStatus *MachineStatus `json:"machineStatus,omitempty"`
	// ControllerStatus holds the status of the firewall-controller reconciling this firewall
	ControllerStatus *ControllerStatus `json:"controllerStatus,omitempty"`
	// Conditions contain the latest available observations of a firewall's current state.
	Conditions Conditions `json:"conditions"`
}

// +kubebuilder:object:root=true
type FirewallMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FirewallMonitor `json:"items"`
}

func (f *FirewallMonitorList) GetItems() []*FirewallMonitor {
	var result []*FirewallMonitor
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
