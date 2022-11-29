package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Firewall is the Schema for the firewalls API
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fw
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.interval`
// +kubebuilder:printcolumn:name="InternalPrefixes",type=string,JSONPath=`.spec.internalprefixes`
// +kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.machineStatus.machineID"
// +kubebuilder:printcolumn:name="Event",type="string",JSONPath=".status.machineStatus.event"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Firewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec     FirewallSpec `json:"spec"`
	Userdata string       `json:"userdata"`

	Status FirewallStatus `json:"status,omitempty"`
}

// FirewallList contains a list of Firewall
// +kubebuilder:object:root=true
type FirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Firewall `json:"items"`
}

// FirewallSpec defines the desired state of Firewall
type FirewallSpec struct {
	// Size is the size for the firewall to be created
	Size string `json:"size"`
	// Image is the os image used for the creation of the firewall
	Image string `json:"image"`
	// PartitionID is the partition in which the firewall gets created
	PartitionID string `json:"partitionID"`
	// ProjectID is the project for which the firewall gets created
	ProjectID string `json:"projectID"`
	// Networks are the networks to which this firewall is connected
	Networks []string `json:"networks"`
	// SSHPublicKeys are the public keys which are added to the firewall's authorized keys file after creation
	SSHPublicKeys []string `json:"sshPublicKeys"`

	// Interval on which rule reconciliation should happen
	Interval string `json:"interval,omitempty"`
	// DryRun if set to true, firewall rules are not applied
	DryRun bool `json:"dryRun,omitempty"`
	// TrafficControl defines where to store the generated ipv4 firewall rules on disk
	Ipv4RuleFile string `json:"ipv4RuleFile,omitempty"`
	// RateLimits allows configuration of rate limit rules for interfaces.
	RateLimits []RateLimit `json:"rateLimits,omitempty"`
	// InternalPrefixes specify prefixes which are considered local to the partition or all regions.
	// Traffic to/from these prefixes is accounted as internal traffic
	InternalPrefixes []string `json:"internalPrefixes,omitempty"`
	// EgressRules contains egress rules configured for this firewall
	EgressRules []EgressRuleSNAT `json:"egressRules,omitempty"`

	// ControllerVersion holds the firewall-controller version to reconcile.
	ControllerVersion string `json:"controllerVersion,omitempty"`
	// ControllerURL points to the downloadable binary artifact of the firewall controller
	ControllerURL string `json:"controllerURL,omitempty"`
	// LogAcceptedConnections if set to true, also log accepted connections in the droptailer log
	LogAcceptedConnections bool `json:"logAcceptedConnections,omitempty"`
}

// EgressRuleSNAT holds a Source-NAT rule
type EgressRuleSNAT struct {
	NetworkID string   `json:"networkID"`
	IPs       []string `json:"ips"`
}

// RateLimit contains the rate limit rule for a network.
type RateLimit struct {
	// NetworkID specifies the network which should be rate limited
	NetworkID string `json:"networkID"`
	// Rate is the input rate in MiB/s
	Rate uint32 `json:"rate"`
}

// FirewallStatus defines the observed state of Firewall
type FirewallStatus struct {
	// MachineStatus holds the status of the firewall machine
	MachineStatus *MachineStatus `json:"machineStatus,omitempty"`

	// ControllerStatus holds the status of the firewall-controller reconciling this firewall
	ControllerStatus *ControllerStatus `json:"controllerStatus,omitempty"`

	// FirewallNetworks holds refined information about the networks that this firewall is connected to
	// the information is used by the firewall-controller in order to reconcile this firewall
	FirewallNetworks []FirewallNetwork `json:"firewallNetworks"`
}

// FirewallConditionType describes the condition types of Firewalls.
type FirewallConditionType string

const (
	// FirewallCreated indicates if the firewall was created at the metal-api
	FirewallCreated FirewallConditionType = "FirewallCreated"
	// FirewallReady indicates that the firewall is running and and according to the metal-api in a healthy, working state
	FirewallReady FirewallConditionType = "FirewallReady"
	// FirewallControllerConnected indicates that the firewall-controller running on the firewall is reconciling the firewall resource
	FirewallControllerConnected FirewallConditionType = "FirewallControllerConnected"
)

// FirewallCondition describes the state of a Firewall at a certain point.
type FirewallCondition struct {
	// Type of Firewall condition.
	Type FirewallConditionType
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus
	// The last time this condition was updated.
	LastTransitionTime metav1.Time
	// The reason for the condition's last transition.
	Reason string
	// A human readable message indicating details about the transition.
	Message string
}

type MachineStatus struct {
	MachineID           string      `json:"machineID,omitempty"`
	Event               string      `json:"event,omitempty"`
	Message             string      `json:"message,omitempty"`
	Liveliness          string      `json:"liveliness,omitempty"`
	EventTimestamp      metav1.Time `json:"eventTimestamp,omitempty"`
	AllocationTimestamp metav1.Time `json:"allocationTimestamp,omitempty"`
	CrashLoop           bool        `json:"crashLoop,omitempty"`
}

type ControllerStatus struct {
	Message           string        `json:"message,omitempty"`
	FirewallStats     FirewallStats `json:"stats"`
	ControllerVersion string        `json:"controllerVersion,omitempty"`
	Updated           metav1.Time   `json:"lastRun,omitempty"`
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
	// Deprecated: TotalBytes is kept for backwards compatibility
	TotalBytes uint64 `json:"total"`
}

type IDSStatsByDevice map[string]InterfaceStat

type InterfaceStat struct {
	Drop             int `json:"drop"`
	InvalidChecksums int `json:"invalidChecksums"`
	Packets          int `json:"packets"`
}

type FirewallNetwork struct {
	Asn                 *int64   `json:"asn"`
	Destinationprefixes []string `json:"destinationPrefixes"`
	Ips                 []string `json:"ips"`
	Nat                 *bool    `json:"nat"`
	Networkid           *string  `json:"networkID"`
	Networktype         *string  `json:"networkType"`
	Prefixes            []string `json:"prefixes"`
	Vrf                 *int64   `json:"vrf"`
}

func (f *FirewallList) GetItems() []*Firewall {
	var result []*Firewall
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
