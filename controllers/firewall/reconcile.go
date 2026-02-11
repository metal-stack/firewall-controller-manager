package firewall

import (
	"fmt"
	"slices"
	"strings"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Reconciler must always return either an error or requeue to ensure that it detects if a firewall get lost etc.
func (c *controller) Reconcile(r *controllers.Ctx[*v2.Firewall]) error {
	var f *models.V1FirewallResponse
	defer func() {
		if err := c.setStatus(r, f); err != nil {
			r.Log.Error(err, "unable to set firewall status")
		}

		mon, err := c.ensureFirewallMonitor(r)
		if err != nil {
			r.Log.Error(err, "unable to deploy firewall monitor")
		}

		SetFirewallStatusFromMonitor(r.Target, mon)
	}()

	fws, err := c.firewallCache.Get(r.Ctx, r.Target)
	if err != nil {
		return controllers.RequeueAfter(10*time.Second, err.Error())
	}

	switch len(fws) {
	case 0:
		r.Target.Status.Phase = v2.FirewallPhaseCreating

		f, err = c.createFirewall(r)
		if err != nil {
			return err
		}

		// requeuing in order to continue checking progression
		return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")
	case 1:
		f = fws[0]

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("Firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
		r.Target.Status.Conditions.Set(cond)

		var currentStatus *v2.MachineStatus
		currentStatus, err = getMachineStatus(f)
		if err != nil {
			r.Log.Error(err, "error finding out machine status")
			return controllers.RequeueAfter(10*time.Second, "error finding out machine status")
		}

		if isFirewallReady(currentStatus) {

			r.Log.Info("firewall reconciled successfully", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionTrue, "Ready", fmt.Sprintf("Firewall %q is phoning home and alive.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			r.Target.Status.Conditions.Set(cond)

			r.Target.Status.Phase = v2.FirewallPhaseRunning

			if err := c.syncTags(r, f); err != nil {
				r.Log.Error(err, "error syncing firewall tags")
				return controllers.RequeueAfter(10*time.Second, "error syncing firewall tags, backing off")
			}

			err := c.syncSSHPubKey(r, f)
			if err != nil {
				return controllers.RequeueAfter(10*time.Second, "error syncing firewall ssh keys, backing off")
			}

			// to make the controller always sync the status with the metal-api, we requeue
			return controllers.RequeueAfter(2*time.Minute, "firewall is running, continue probing regularly for status sync")

		} else if isFirewallProgressing(currentStatus) {

			r.Log.Info("firewall is progressing", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotReady", fmt.Sprintf("Firewall %q is not ready.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			r.Target.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")

		} else {

			r.Log.Error(fmt.Errorf("firewall is not finishing the provisioning"), "please investigate", "id", pointer.SafeDeref(f.ID))

			if pointer.SafeDeref(currentStatus).CrashLoop {
				r.Target.Status.Phase = v2.FirewallPhaseCrashing
			}

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotFinishing", fmt.Sprintf("Firewall %q is not finishing the provisioning procedure.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			r.Target.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(1*time.Minute, "firewall creation is not finishing, proceed probing")

		}
	default:
		var ids []string
		for _, fw := range fws {
			ids = append(ids, pointer.SafeDeref(fw.ID))
		}

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "MultipleFirewalls", fmt.Sprintf("Found multiple firewalls with the same name: %s", strings.Join(ids, ", ")))
		r.Target.Status.Conditions.Set(cond)

		return controllers.RequeueAfter(1*time.Minute, "multiple firewalls found with the same name, please investigate")
	}
}

func (c *controller) createFirewall(r *controllers.Ctx[*v2.Firewall]) (*models.V1FirewallResponse, error) {
	var (
		networks []*models.V1MachineAllocationNetwork
		tags     = []string{
			c.c.GetClusterTag(),
			v2.FirewallManagedByTag(),
		}
	)
	for _, n := range r.Target.Spec.Networks {
		network := &models.V1MachineAllocationNetwork{
			Networkid:   &n,
			Autoacquire: new(true),
		}
		networks = append(networks, network)
	}

	ref := metav1.GetControllerOf(r.Target)
	if ref != nil {
		tags = append(tags, v2.FirewallSetTag(ref.Name))
	}

	createRequest := &models.V1FirewallCreateRequest{
		Description: "created by firewall-controller-manager",
		Name:        r.Target.Name,
		Hostname:    r.Target.Name,
		Sizeid:      &r.Target.Spec.Size,
		Projectid:   &r.Target.Spec.Project,
		Partitionid: &r.Target.Spec.Partition,
		Imageid:     &r.Target.Spec.Image,
		SSHPubKeys:  r.Target.Spec.SSHPublicKeys,
		Networks:    networks,
		UserData:    r.Target.Spec.Userdata,
		Tags:        tags,
	}

	resp, err := c.c.GetMetal().Firewall().AllocateFirewall(firewall.NewAllocateFirewallParams().WithBody(createRequest).WithContext(r.Ctx), nil)
	if err != nil {
		r.Log.Error(err, "error creating firewall")

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "NotCreated", fmt.Sprintf("Firewall could not be created: %s.", err))
		r.Target.Status.Conditions.Set(cond)

		return nil, controllers.RequeueAfter(30*time.Second, "error creating firewall, backing off")
	}

	r.Log.Info("firewall created", "id", pointer.SafeDeref(resp.Payload.ID))

	cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("Firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(resp.Payload.Allocation).Name)))
	r.Target.Status.Conditions.Set(cond)

	c.recorder.Eventf(r.Target, nil, "Normal", "Create", "created firewall %s id %s", r.Target.Name, pointer.SafeDeref(resp.Payload.ID))

	return resp.Payload, nil
}

func isFirewallProgressing(status *v2.MachineStatus) bool {
	if status == nil || status.LastEvent == nil {
		return false
	}
	if status.CrashLoop {
		return false
	}
	// TODO replace with models.V1LivelinessAlive once merged
	if status.Liveliness != "Alive" {
		return false
	}
	// TODO replace with models.V1LivelinessPhonedHome once merged
	if status.LastEvent.Event != "Phoned Home" {
		return true
	}
	return false
}

func isFirewallReady(status *v2.MachineStatus) bool {
	if status == nil || status.LastEvent == nil {
		return false
	}
	if status.CrashLoop {
		return false
	}
	// TODO replace with models.V1LivelinessAlive once merged
	if status.Liveliness != "Alive" {
		return false
	}
	// TODO replace with models.V1LivelinessPhonedHome once merged
	if status.LastEvent.Event == "Phoned Home" {
		return true
	}
	return false
}

func (c *controller) syncTags(r *controllers.Ctx[*v2.Firewall], m *models.V1FirewallResponse) error {
	var (
		newTags []string
		ref     = metav1.GetControllerOf(r.Target)
	)

	newTags = ensureTag(m.Tags, v2.FirewallControllerManagedByAnnotation, v2.FirewallControllerManager)
	if ref != nil {
		newTags = ensureTag(newTags, v2.FirewallControllerSetAnnotation, ref.Name)
	}

	if sets.NewString(newTags...).Equal(sets.NewString(m.Tags...)) {
		return nil
	}

	_, err := c.c.GetMetal().Machine().UpdateMachine(machine.NewUpdateMachineParams().WithBody(&models.V1MachineUpdateRequest{
		ID:   m.ID,
		Tags: newTags,
	}).WithContext(r.Ctx), nil)
	if err != nil {
		return err
	}

	return nil
}

func ensureTag(currentTags []string, key, value string) []string {
	var (
		newTags  []string
		foundTag = false
	)

	for _, tag := range currentTags {
		k, _, found := strings.Cut(tag, "=")
		if !found {
			newTags = append(newTags, tag)
			continue
		}

		if k != key {
			newTags = append(newTags, tag)
			continue
		}

		foundTag = true

		newTags = append(newTags, fmt.Sprintf("%s=%s", key, value))
	}

	if !foundTag {
		newTags = append(newTags, fmt.Sprintf("%s=%s", key, value))
	}

	return newTags
}

func (c *controller) syncSSHPubKey(r *controllers.Ctx[*v2.Firewall], m *models.V1FirewallResponse) error {
	if m.Allocation == nil {
		return fmt.Errorf("firewall has no allocation in metal-api")
	}

	if slices.Equal(m.Allocation.SSHPubKeys, r.Target.Spec.SSHPublicKeys) {
		return nil
	}

	_, err := c.c.GetMetal().Machine().UpdateMachine(machine.NewUpdateMachineParams().WithBody(&models.V1MachineUpdateRequest{
		ID:         m.ID,
		SSHPubKeys: r.Target.Spec.SSHPublicKeys,
	}).WithContext(r.Ctx), nil)
	if err != nil {
		return fmt.Errorf("unable to update ssh public keys: %w", err)
	}

	r.Log.Info("updated changed ssh public keys")

	return nil
}
