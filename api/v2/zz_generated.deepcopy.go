//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v2

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControllerStatus) DeepCopyInto(out *ControllerStatus) {
	*out = *in
	in.FirewallStats.DeepCopyInto(&out.FirewallStats)
	in.Updated.DeepCopyInto(&out.Updated)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControllerStatus.
func (in *ControllerStatus) DeepCopy() *ControllerStatus {
	if in == nil {
		return nil
	}
	out := new(ControllerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Counter) DeepCopyInto(out *Counter) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Counter.
func (in *Counter) DeepCopy() *Counter {
	if in == nil {
		return nil
	}
	out := new(Counter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeviceStat) DeepCopyInto(out *DeviceStat) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeviceStat.
func (in *DeviceStat) DeepCopy() *DeviceStat {
	if in == nil {
		return nil
	}
	out := new(DeviceStat)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in DeviceStatsByDevice) DeepCopyInto(out *DeviceStatsByDevice) {
	{
		in := &in
		*out = make(DeviceStatsByDevice, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeviceStatsByDevice.
func (in DeviceStatsByDevice) DeepCopy() DeviceStatsByDevice {
	if in == nil {
		return nil
	}
	out := new(DeviceStatsByDevice)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EgressRuleSNAT) DeepCopyInto(out *EgressRuleSNAT) {
	*out = *in
	if in.IPs != nil {
		in, out := &in.IPs, &out.IPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EgressRuleSNAT.
func (in *EgressRuleSNAT) DeepCopy() *EgressRuleSNAT {
	if in == nil {
		return nil
	}
	out := new(EgressRuleSNAT)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Firewall) DeepCopyInto(out *Firewall) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Firewall.
func (in *Firewall) DeepCopy() *Firewall {
	if in == nil {
		return nil
	}
	out := new(Firewall)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Firewall) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallCondition) DeepCopyInto(out *FirewallCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallCondition.
func (in *FirewallCondition) DeepCopy() *FirewallCondition {
	if in == nil {
		return nil
	}
	out := new(FirewallCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallDeployment) DeepCopyInto(out *FirewallDeployment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallDeployment.
func (in *FirewallDeployment) DeepCopy() *FirewallDeployment {
	if in == nil {
		return nil
	}
	out := new(FirewallDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FirewallDeployment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallDeploymentList) DeepCopyInto(out *FirewallDeploymentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FirewallDeployment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallDeploymentList.
func (in *FirewallDeploymentList) DeepCopy() *FirewallDeploymentList {
	if in == nil {
		return nil
	}
	out := new(FirewallDeploymentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FirewallDeploymentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallDeploymentSpec) DeepCopyInto(out *FirewallDeploymentSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallDeploymentSpec.
func (in *FirewallDeploymentSpec) DeepCopy() *FirewallDeploymentSpec {
	if in == nil {
		return nil
	}
	out := new(FirewallDeploymentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallDeploymentStatus) DeepCopyInto(out *FirewallDeploymentStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallDeploymentStatus.
func (in *FirewallDeploymentStatus) DeepCopy() *FirewallDeploymentStatus {
	if in == nil {
		return nil
	}
	out := new(FirewallDeploymentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallList) DeepCopyInto(out *FirewallList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Firewall, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallList.
func (in *FirewallList) DeepCopy() *FirewallList {
	if in == nil {
		return nil
	}
	out := new(FirewallList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FirewallList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallNetwork) DeepCopyInto(out *FirewallNetwork) {
	*out = *in
	if in.Asn != nil {
		in, out := &in.Asn, &out.Asn
		*out = new(int64)
		**out = **in
	}
	if in.Destinationprefixes != nil {
		in, out := &in.Destinationprefixes, &out.Destinationprefixes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Ips != nil {
		in, out := &in.Ips, &out.Ips
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Nat != nil {
		in, out := &in.Nat, &out.Nat
		*out = new(bool)
		**out = **in
	}
	if in.Networkid != nil {
		in, out := &in.Networkid, &out.Networkid
		*out = new(string)
		**out = **in
	}
	if in.Networktype != nil {
		in, out := &in.Networktype, &out.Networktype
		*out = new(string)
		**out = **in
	}
	if in.Prefixes != nil {
		in, out := &in.Prefixes, &out.Prefixes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Vrf != nil {
		in, out := &in.Vrf, &out.Vrf
		*out = new(int64)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallNetwork.
func (in *FirewallNetwork) DeepCopy() *FirewallNetwork {
	if in == nil {
		return nil
	}
	out := new(FirewallNetwork)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallSet) DeepCopyInto(out *FirewallSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallSet.
func (in *FirewallSet) DeepCopy() *FirewallSet {
	if in == nil {
		return nil
	}
	out := new(FirewallSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FirewallSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallSetList) DeepCopyInto(out *FirewallSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FirewallSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallSetList.
func (in *FirewallSetList) DeepCopy() *FirewallSetList {
	if in == nil {
		return nil
	}
	out := new(FirewallSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FirewallSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallSetSpec) DeepCopyInto(out *FirewallSetSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallSetSpec.
func (in *FirewallSetSpec) DeepCopy() *FirewallSetSpec {
	if in == nil {
		return nil
	}
	out := new(FirewallSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallSetStatus) DeepCopyInto(out *FirewallSetStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallSetStatus.
func (in *FirewallSetStatus) DeepCopy() *FirewallSetStatus {
	if in == nil {
		return nil
	}
	out := new(FirewallSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallSpec) DeepCopyInto(out *FirewallSpec) {
	*out = *in
	if in.Networks != nil {
		in, out := &in.Networks, &out.Networks
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SSHPublicKeys != nil {
		in, out := &in.SSHPublicKeys, &out.SSHPublicKeys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RateLimits != nil {
		in, out := &in.RateLimits, &out.RateLimits
		*out = make([]RateLimit, len(*in))
		copy(*out, *in)
	}
	if in.InternalPrefixes != nil {
		in, out := &in.InternalPrefixes, &out.InternalPrefixes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EgressRules != nil {
		in, out := &in.EgressRules, &out.EgressRules
		*out = make([]EgressRuleSNAT, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallSpec.
func (in *FirewallSpec) DeepCopy() *FirewallSpec {
	if in == nil {
		return nil
	}
	out := new(FirewallSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallStats) DeepCopyInto(out *FirewallStats) {
	*out = *in
	if in.RuleStats != nil {
		in, out := &in.RuleStats, &out.RuleStats
		*out = make(RuleStatsByAction, len(*in))
		for key, val := range *in {
			var outVal map[string]RuleStat
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make(RuleStats, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
	if in.DeviceStats != nil {
		in, out := &in.DeviceStats, &out.DeviceStats
		*out = make(DeviceStatsByDevice, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.IDSStats != nil {
		in, out := &in.IDSStats, &out.IDSStats
		*out = make(IDSStatsByDevice, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallStats.
func (in *FirewallStats) DeepCopy() *FirewallStats {
	if in == nil {
		return nil
	}
	out := new(FirewallStats)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FirewallStatus) DeepCopyInto(out *FirewallStatus) {
	*out = *in
	if in.MachineStatus != nil {
		in, out := &in.MachineStatus, &out.MachineStatus
		*out = new(MachineStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerStatus != nil {
		in, out := &in.ControllerStatus, &out.ControllerStatus
		*out = new(ControllerStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.FirewallNetworks != nil {
		in, out := &in.FirewallNetworks, &out.FirewallNetworks
		*out = make([]FirewallNetwork, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FirewallStatus.
func (in *FirewallStatus) DeepCopy() *FirewallStatus {
	if in == nil {
		return nil
	}
	out := new(FirewallStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in IDSStatsByDevice) DeepCopyInto(out *IDSStatsByDevice) {
	{
		in := &in
		*out = make(IDSStatsByDevice, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IDSStatsByDevice.
func (in IDSStatsByDevice) DeepCopy() IDSStatsByDevice {
	if in == nil {
		return nil
	}
	out := new(IDSStatsByDevice)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InterfaceStat) DeepCopyInto(out *InterfaceStat) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InterfaceStat.
func (in *InterfaceStat) DeepCopy() *InterfaceStat {
	if in == nil {
		return nil
	}
	out := new(InterfaceStat)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineStatus) DeepCopyInto(out *MachineStatus) {
	*out = *in
	in.EventTimestamp.DeepCopyInto(&out.EventTimestamp)
	in.AllocationTimestamp.DeepCopyInto(&out.AllocationTimestamp)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineStatus.
func (in *MachineStatus) DeepCopy() *MachineStatus {
	if in == nil {
		return nil
	}
	out := new(MachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RateLimit) DeepCopyInto(out *RateLimit) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RateLimit.
func (in *RateLimit) DeepCopy() *RateLimit {
	if in == nil {
		return nil
	}
	out := new(RateLimit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleStat) DeepCopyInto(out *RuleStat) {
	*out = *in
	out.Counter = in.Counter
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleStat.
func (in *RuleStat) DeepCopy() *RuleStat {
	if in == nil {
		return nil
	}
	out := new(RuleStat)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in RuleStats) DeepCopyInto(out *RuleStats) {
	{
		in := &in
		*out = make(RuleStats, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleStats.
func (in RuleStats) DeepCopy() RuleStats {
	if in == nil {
		return nil
	}
	out := new(RuleStats)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in RuleStatsByAction) DeepCopyInto(out *RuleStatsByAction) {
	{
		in := &in
		*out = make(RuleStatsByAction, len(*in))
		for key, val := range *in {
			var outVal map[string]RuleStat
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make(RuleStats, len(*in))
				for key, val := range *in {
					(*out)[key] = val
				}
			}
			(*out)[key] = outVal
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleStatsByAction.
func (in RuleStatsByAction) DeepCopy() RuleStatsByAction {
	if in == nil {
		return nil
	}
	out := new(RuleStatsByAction)
	in.DeepCopyInto(out)
	return *out
}
