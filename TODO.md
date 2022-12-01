# TODO's

- [x] Manual deletion of FirewallDeployment/FirewallSet/Firewall is not possible
  - A user can now roll a firewall set through a firewall monitor resource in his cluster
- [ ] Define how Migration of clusters with a v1.Firewall to v2.Firewall can be done
  - [ ] Do not implement a migration, just support creating a new v2.FirewallDeployment which replaces the v1.Firewall Object and FirewallMachine after successful creation of a v2.Firewall
  - [ ] InPlace migrate v1.Firewall to a full set of v2.[FirewallDeployment/FirewallSet/Firewall] ( for sure very difficult and dangerous )
  - [ ] other Idea
- [ ] Adopt Gardener Extension Provider
- [ ] Adopt Firewall Controller to consume v2.Firewall from Seed and write v2.ControllerStatus to Shoot.
- [ ] Adopt Integration Test Suite
- [ ] Remove IngressRules from CWNP, only rely on Service.Spec.LoadBalancerSourceRanges
