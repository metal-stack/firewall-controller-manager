package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	// FirewallSeedURLAnnotation contains information on the location of the seed's api server endpoint. this is used
	// for gardener shoot migrations, which will change the seed api server endpoint for the firewall-controller. the
	// firewall-controller can use this annotation to update it's kubeconfig accordingly.
	FirewallSeedURLAnnotation = "firewall.metal-stack.io/seed-api-url"

	// FirewallShootNamespace is the name of the namespace to which the firewall monitor gets deployed and in which the firewall-controller operates
	FirewallShootNamespace = "firewall"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fwmon
// +kubebuilder:printcolumn:name="Machine ID",type="string",JSONPath=".machineStatus.machineID"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".image"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".size"
// +kubebuilder:printcolumn:name="Last Event",type="string",JSONPath=".machineStatus.lastEvent.event"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".machineStatus.allocationTimestamp"
//
// FirewallMonitor is typically deployed into the shoot cluster in comparison to the other resources of this controller
// which are deployed into the seed cluster's shoot namespace.
type FirewallMonitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Size is the machine size of the firewall.
	Size string `json:"size"`
	// Image is the os image of the firewall.
	Image string `json:"image"`
	// Partition is the partition in which the firewall resides.
	Partition string `json:"partition"`
	// Project is the project in which the firewall resides.
	Project string `json:"project"`
	// Networks are the networks to which this firewall is connected.
	Networks []string `json:"networks"`

	// RateLimits allows configuration of rate limit rules for interfaces.
	RateLimits []RateLimit `json:"rateLimits,omitempty"`
	// EgressRules contains egress rules configured for this firewall.
	EgressRules []EgressRuleSNAT `json:"egressRules,omitempty"`
	// LogAcceptedConnections if set to true, also log accepted connections in the droptailer log.
	LogAcceptedConnections bool `json:"logAcceptedConnections,omitempty"`

	// MachineStatus holds the status of the firewall machine
	MachineStatus *MachineStatus `json:"machineStatus,omitempty"`
	// ControllerStatus holds the status of the firewall-controller reconciling this firewall
	ControllerStatus *ControllerStatus `json:"controllerStatus,omitempty"`
	// Conditions contain the latest available observations of a firewall's current state.
	Conditions Conditions `json:"conditions"`
}

type ControllerStatus struct {
	Message                 string           `json:"message,omitempty"`
	FirewallStats           *FirewallStats   `json:"stats,omitempty"`
	ControllerVersion       string           `json:"controllerVersion,omitempty"`
	NftablesExporterVersion string           `json:"nftablesExporterVersion,omitempty"`
	Updated                 metav1.Time      `json:"lastRun,omitempty"`
	Distance                FirewallDistance `json:"distance,omitempty"`
	DistanceSupported       bool             `json:"distanceSupported,omitempty"`
}

// FirewallStats contains firewall statistics
type FirewallStats struct {
	RuleStats   RuleStatsByAction   `json:"rules"`
	DeviceStats DeviceStatsByDevice `json:"devices"`
	IDSStats    IDSStatsByDevice    `json:"idsStats"`
}

// RuleStatsByAction contains firewall rule statistics groups by action: e.g. accept, drop, policy, masquerade
type RuleStatsByAction map[string]RuleStats

// RuleStats contains firewall rule statistics of all rules of an action
type RuleStats map[string]RuleStat

// RuleStat contains the statistics for a single nftables rule
type RuleStat struct {
	Counter Counter `json:"counter"`
}

// Counter holds values of a nftables counter object
type Counter struct {
	Bytes   uint64 `json:"bytes"`
	Packets uint64 `json:"packets"`
}

// DeviceStatsByDevice contains DeviceStatistics grouped by device name
type DeviceStatsByDevice map[string]DeviceStat

// DeviceStat contains statistics of a device
type DeviceStat struct {
	InBytes  uint64 `json:"in"`
	OutBytes uint64 `json:"out"`
}

type IDSStatsByDevice map[string]InterfaceStat

type InterfaceStat struct {
	Drop             int `json:"drop"`
	InvalidChecksums int `json:"invalidChecksums"`
	Packets          int `json:"packets"`
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
