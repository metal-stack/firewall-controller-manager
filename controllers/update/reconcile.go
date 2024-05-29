package update

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	return c.autoUpdateOS(r)
}

func (c *controller) autoUpdateOS(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	if !r.Target.Spec.AutoUpdate.MachineImage {
		return nil
	}

	if !r.WithinMaintenance {
		c.log.Info("not checking for newer os image, not in maintenance time window")
		return nil
	}

	c.log.Info("checking for newer os image")

	// first, let's resolve the latest image from the api

	os, version, err := getOsAndSemverFromImage(r.Target.Spec.Template.Spec.Image)
	if err != nil {
		return fmt.Errorf("image version cannot be parsed: %w", err)
	}

	var (
		imageToResolve                = r.Target.Spec.Template.Spec.Image
		isFullyQualifiedImageNotation = version.Patch() != 0
	)

	if isFullyQualifiedImageNotation {
		imageToResolve = fmt.Sprintf("%s-%d.%d", os, version.Major(), version.Minor())
	}

	image, err := c.imageCache.Get(r.Ctx, imageToResolve)
	if err != nil {
		return fmt.Errorf("unable to retrieve latest os image from metal-api: %w", err)
	}

	if image.ID == nil {
		return fmt.Errorf("returned image from metal-api contains no id")
	}

	// now figure out the actual running image of the current firewall
	// this can be different from specification in case a shorthand image notation is being used
	// so we need the exact version running

	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	latestSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	ownedFirewalls, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, latestSet, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	v2.SortFirewallsByImportance(ownedFirewalls)

	if len(ownedFirewalls) == 0 {
		return nil
	}

	fw := ownedFirewalls[0] // this is the currently active one

	if fw.Status.MachineStatus == nil || fw.Status.MachineStatus.ImageID == "" {
		return nil
	}

	// finally, we can do the image comparison

	if *image.ID == fw.Status.MachineStatus.ImageID {
		r.Log.Info("no new os version available, not triggering auto-update")
		return nil
	}

	r.Log.Info("newer os version is available, triggering auto-update")

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		refetched := &v2.FirewallDeployment{}
		err := c.c.GetSeedClient().Get(r.Ctx, client.ObjectKeyFromObject(r.Target), refetched)
		if err != nil {
			return fmt.Errorf("unable re-fetch firewall deployment: %w", err)
		}

		if refetched.Annotations == nil {
			refetched.Annotations = map[string]string{}
		}

		refetched.Annotations[v2.RollSetAnnotation] = strconv.FormatBool(true)
		if isFullyQualifiedImageNotation {
			refetched.Spec.Template.Spec.Image = *image.ID
		}

		err = c.c.GetSeedClient().Update(r.Ctx, refetched)
		if err != nil {
			return fmt.Errorf("unable to update firewall deployment: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// copied over from metal-api because this is only available in internal package
func getOsAndSemverFromImage(id string) (string, *semver.Version, error) {
	imageParts := strings.Split(id, "-")
	if len(imageParts) < 2 {
		return "", nil, errors.New("image does not contain a version")
	}

	parts := len(imageParts) - 1
	os := strings.Join(imageParts[:parts], "-")
	version := strings.Join(imageParts[parts:], "")
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", nil, err
	}
	return os, v, nil
}
