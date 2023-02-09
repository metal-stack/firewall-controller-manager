package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// FirewallNoControllerConnectionAnnotation can be used as an annotation to the firewall resource in order
	// to indicate that the firewall-controller does not connect to the firewall monitor. this way, the replica
	// set will become healthy without a controller connection.
	//
	// useful for the migration when having old firewall v1 controllers that cannot update the monitor.
	FirewallNoControllerConnectionAnnotation = "firewall.metal-stack.io/no-controller-connection"
	// FirewallControllerManagedByAnnotation is used as tag for creating a firewall to indicate who is managing the firewall.
	FirewallControllerManagedByAnnotation = "firewall.metal-stack.io/managed-by"
	// FirewallControllerManager is a name of the firewall-controller-manager managing the firewall.
	FirewallControllerManager = "firewall-controller-manager"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Firewall represents a metal-stack firewall in a bare-metal kubernetes cluster. It has a 1:1 relationship to a firewall in the metal-stack api.
//
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fw
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Machine ID",type="string",JSONPath=".status.machineStatus.machineID"
// +kubebuilder:printcolumn:name="Last Event",type="string",JSONPath=".status.machineStatus.lastEvent.event"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".status.machineStatus.allocationTimestamp"
type Firewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the firewall specification.
	Spec FirewallSpec `json:"spec"`
	// Status contains current status information on the firewall.
	Status FirewallStatus `json:"status,omitempty"`
}

// FirewallSpec defines parameters for the firewall creation along with configuration for the firewall-controller.
type FirewallSpec struct {
	// Size is the machine size of the firewall.
	// An update on this field requires the recreation of the physical firewall and can therefore lead to traffic interruption for the cluster.
	Size string `json:"size"`
	// Image is the os image of the firewall.
	// An update on this field requires the recreation of the physical firewall and can therefore lead to traffic interruption for the cluster.
	Image string `json:"image"`
	// Partition is the partition in which the firewall resides.
	Partition string `json:"partition"`
	// Project is the project in which the firewall resides.
	Project string `json:"project"`
	// Networks are the networks to which this firewall is connected.
	// An update on this field requires the recreation of the physical firewall and can therefore lead to traffic interruption for the cluster.
	Networks []string `json:"networks"`

	// Userdata contains the userdata used for the creation of the firewall.
	// It gets defaulted to a userdata matching for the firewall-controller with connection to Gardener shoot and seed.
	Userdata string `json:"userdata,omitempty"`
	// SSHPublicKeys are public keys which are added to the firewall's authorized keys file on creation.
	// It gets defaulted to the public key of ssh secret as provided by the controller flags.
	SSHPublicKeys []string `json:"sshPublicKeys,omitempty"`

	// RateLimits allows configuration of rate limit rules for interfaces.
	RateLimits []RateLimit `json:"rateLimits,omitempty"`
	// InternalPrefixes specify prefixes which are considered local to the partition or all regions. This is used for the traffic counters.
	// Traffic to/from these prefixes is counted as internal traffic.
	InternalPrefixes []string `json:"internalPrefixes,omitempty"`
	// EgressRules contains egress rules configured for this firewall.
	EgressRules []EgressRuleSNAT `json:"egressRules,omitempty"`

	// Interval on which rule reconciliation by the firewall-controller should happen.
	Interval string `json:"interval,omitempty"`
	// DryRun if set to true, firewall rules are not applied. For devel-purposes only.
	DryRun bool `json:"dryRun,omitempty"`
	// TrafficControl defines where to store the generated ipv4 firewall rules on disk.
	Ipv4RuleFile string `json:"ipv4RuleFile,omitempty"`
	// ControllerVersion holds the firewall-controller version to reconcile.
	ControllerVersion string `json:"controllerVersion,omitempty"`
	// ControllerURL points to the downloadable binary artifact of the firewall controller
	ControllerURL string `json:"controllerURL,omitempty"`
	// LogAcceptedConnections if set to true, also log accepted connections in the droptailer log.
	LogAcceptedConnections bool `json:"logAcceptedConnections,omitempty"`
	// DNSServerAddress specifies DNS server address used by DNS proxy
	DNSServerAddress string `json:"dnsServerAddress,omitempty"`
	// DNSPort specifies port to which DNS proxy should be bound
	DNSPort *uint `json:"dnsPort,omitempty"`
}

// FirewallTemplateSpec describes the data a firewall should have when created from a template
type FirewallTemplateSpec struct {
	// Metadata of the firewalls created from this template.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the firewall specification.
	Spec FirewallSpec `json:"spec,omitempty"`
}

// EgressRuleSNAT holds a Source-NAT rule
type EgressRuleSNAT struct {
	// NetworkID is the network for which the egress rule will be configured.
	NetworkID string `json:"networkID"`
	// IPs contains the ips used as source addresses for packets leaving the specified network.
	IPs []string `json:"ips"`
}

// RateLimit contains the rate limit rule for a network.
type RateLimit struct {
	// NetworkID specifies the network which should be rate limited.
	NetworkID string `json:"networkID"`
	// Rate is the input rate in MiB/s.
	Rate uint32 `json:"rate"`
}

// FirewallStatus contains current status information on the firewall.
type FirewallStatus struct {
	// MachineStatus holds the status of the firewall machine containing information from the metal-stack api.
	MachineStatus *MachineStatus `json:"machineStatus,omitempty"`
	// ControllerStatus holds the a brief version of the firewall-controller reconciling this firewall.
	ControllerStatus *ControllerConnection `json:"controllerStatus,omitempty"`
	// FirewallNetworks holds refined information about the networks that this firewall is connected to.
	// The information is used by the firewall-controller in order to reconcile this firewall.
	FirewallNetworks []FirewallNetwork `json:"firewallNetworks,omitempty"`
	// Conditions contain the latest available observations of a firewall's current state.
	Conditions Conditions `json:"conditions"`
	// Phase describes the firewall phase at the current time.
	Phase FirewallPhase `json:"phase"`
	// ShootAccess contains references to construct shoot clients.
	ShootAccess *ShootAccess `json:"shootAccess,omitempty"`
}

// FirewallPhase describes the firewall phase at the current time.
type FirewallPhase string

const (
	// FirewallPhaseCreating means the firewall is currently being created.
	FirewallPhaseCreating FirewallPhase = "Creating"
	// FirewallPhaseRunning means the firewall is currently running.
	FirewallPhaseRunning FirewallPhase = "Running"
	// FirewallPhaseCrashing means the firewall is currently in a provisioning crashloop.
	FirewallPhaseCrashing FirewallPhase = "Crashing"
)

const (
	// FirewallCreated indicates if the firewall was created at the metal-api
	FirewallCreated ConditionType = "Created"
	// FirewallReady indicates that the firewall is running and and according to the metal-api in a healthy, working state
	FirewallReady ConditionType = "Ready"
	// FirewallControllerConnected indicates that the firewall-controller running on the firewall is reconciling the firewall resource
	FirewallControllerConnected ConditionType = "Connected"
	// FirewallMonitorDeployed indicates that the firewall monitor is deployed into the shoot cluster
	FirewallMonitorDeployed ConditionType = "MonitorDeployed"
)

// ShootAccess contains secret references to construct a shoot client in the firewall-controller to update its firewall monitor.
//
// The controller has to be aware that Gardener will rotate these secrets on regular basis so it has to exchange
// the client when the access expires.
type ShootAccess struct {
	// GenericKubeconfigSecretName is the secret name of the generic kubeconfig secret deployed by Gardener
	// to be used as a template for constructing a shoot client.
	GenericKubeconfigSecretName string `json:"genericKubeconfigSecretName"`
	// TokenSecretName is the secret name for the access token for shoot access.
	TokenSecretName string `json:"tokenSecretName"`
	// Namespace is the namespace in the seed where the secrets reside.
	Namespace string `json:"namespace"`
	// APIServerURL is the URL of the shoot's API server.
	APIServerURL string `json:"apiServerURL"`
	// SSHKeySecretName is the secret name containing the ssh keys for the machine access.
	SSHKeySecretName string `json:"sshKeySecretName"`
}

// MachineStatus holds the status of the firewall machine containing information from the metal-stack api.
type MachineStatus struct {
	// MachineID is the id of the firewall in the metal-stack api.
	MachineID string `json:"machineID"`
	// AllocationTimestamp is the timestamp when the machine was allocated.
	AllocationTimestamp metav1.Time `json:"allocationTimestamp"`
	// Liveliness expresses the liveliness of the firewall and can be used to determine the general health state of the machine.
	Liveliness string `json:"liveliness"`
	// CrashLoop can occur during provisioning of the firewall causing the firewall not to get ready.
	CrashLoop bool `json:"crashLoop,omitempty"`
	// LastEvent contains the last provisioning event of the machine.
	LastEvent *MachineLastEvent `json:"lastEvent,omitempty"`
}

// MachineLastEvent contains the last provisioning event of the machine.
type MachineLastEvent struct {
	// Event is the provisioning event.
	Event string `json:"event"`
	// Timestamp is the point in time when the provisioning event was received.
	Timestamp metav1.Time `json:"timestamp"`
	// Message contains a message further describing the event.
	Message string `json:"message"`
}

// ControllerConnection contains information about the firewall-controller connection.
type ControllerConnection struct {
	// ActualVersion is the actual version running at the firewall-controller.
	ActualVersion string `json:"actualVersion,omitempty"`
	// Updated is a timestamp when the controller has last reconciled the firewall resource.
	Updated metav1.Time `json:"lastRun,omitempty"`
}

// FirewallNetwork holds refined information about a network that the firewall is connected to.
// The information is used by the firewall-controller in order to reconcile the firewall.
type FirewallNetwork struct {
	// Asn is the autonomous system number of this network.
	ASN *int64 `json:"asn"`
	// DestinationPrefixes are the destination prefixes of this network.
	DestinationPrefixes []string `json:"destinationPrefixes"`
	// IPs are the ip addresses used in this network.
	IPs []string `json:"ips"`
	// Nat specifies whether the outgoing traffic is natted or not.
	Nat *bool `json:"nat"`
	// NetworkID is the id of this network.
	NetworkID *string `json:"networkID"`
	// NetworkType is the type of this network.
	NetworkType *string `json:"networkType"`
	// Prefixes are the network prefixes of this network.
	Prefixes []string `json:"prefixes"`
	// Vrf is vrf id of this network.
	Vrf *int64 `json:"vrf"`
}

// FirewallList contains a list of firewalls
//
// +kubebuilder:object:root=true
type FirewallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items contains the list items.
	Items []Firewall `json:"items"`
}

func (f *FirewallList) GetItems() []*Firewall {
	var result []*Firewall
	for i := range f.Items {
		result = append(result, &f.Items[i])
	}
	return result
}
