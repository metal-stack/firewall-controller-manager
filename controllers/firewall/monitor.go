package firewall

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	firewallBootstrapTokenIDLabel           = "firewall.metal-stack.io/bootstrap-token-id"
	firewallBootstrapTokenNextRotationLabel = "firewall.metal-stack.io/bootstrap-token-next-rotation"
	characterSetResourceNameFragment        = "abcdefghijklmnopqrstuvwxyz0123456789"
	firewallBootstrapTokenExpiration        = 20 * time.Minute
	firewallBootstrapTokenRotationPeriod    = 15 * time.Minute
	// Extra groups to authenticate the token as. Must start with "system:bootstrappers:"
	firewallBootstrapTokenAuthExtraGroups = ""
)

func (c *controller) ensureFirewallMonitor(r *controllers.Ctx[*v2.Firewall]) (*v2.FirewallMonitor, error) {
	var err error

	defer func() {
		if err != nil {
			r.Log.Error(err, "error deploying firewall monitor")

			cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionFalse, "NotDeployed", fmt.Sprintf("Monitor could not be deployed: %s", err))
			r.Target.Status.Conditions.Set(cond)

			return
		}

		r.Log.Info("firewall monitor deployed")

		cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionTrue, "Deployed", "Successfully deployed firewall-monitor.")
		r.Target.Status.Conditions.Set(cond)
	}()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.c.GetShootNamespace(),
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.c.GetShootClient(), ns, func() error {
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure namespace for monitor resource: %w", err)
	}

	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.c.GetShootNamespace(),
		},
	}

	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.c.GetShootClient(), mon, func() error {
		mon.Size = r.Target.Spec.Size
		mon.Image = r.Target.Spec.Image
		mon.Partition = r.Target.Spec.Partition
		mon.Project = r.Target.Spec.Project
		mon.Networks = r.Target.Spec.Networks
		mon.RateLimits = r.Target.Spec.RateLimits
		mon.EgressRules = r.Target.Spec.EgressRules
		mon.LogAcceptedConnections = r.Target.Spec.LogAcceptedConnections
		mon.MachineStatus = r.Target.Status.MachineStatus
		mon.Conditions = r.Target.Status.Conditions
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure firewall monitor resource: %w", err)
	}

	return mon, nil
}

func (c *controller) rotateFirewallBootstrapTokenIfNeeded(r *controllers.Ctx[*v2.Firewall]) error {
	var (
		tokenID         = r.Target.Labels[firewallBootstrapTokenIDLabel]
		nextRotationStr = r.Target.Labels[firewallBootstrapTokenNextRotationLabel]
	)

	if tokenID == "" || nextRotationStr == "" {
		return nil
	}

	nextRotation, err := time.Parse(time.RFC3339, nextRotationStr)
	if err == nil && (nextRotation.Equal(time.Now()) || nextRotation.Before(time.Now())) {
		r.Log.Info("bootstrap token rotation not needed")
		return nil
	}
	if err != nil {
		r.Log.Info("invalid firewall bootstrap token interval, %s", err)
	}

	r.Log.Info("rotate bootstrap token for firewall deployment %q", client.ObjectKeyFromObject(r.Target))
	tokenID, err = generateNewTokenID()
	if err != nil {
		return err
	}
	nextRotation = time.Now().Add(firewallBootstrapTokenRotationPeriod)
	nextRotationStr = nextRotation.Format(time.RFC3339)

	r.Target.Labels[firewallBootstrapTokenNextRotationLabel] = nextRotationStr
	r.Target.Labels[firewallBootstrapTokenIDLabel] = tokenID

	secretName := bootstraptokenutil.BootstrapTokenSecretName(tokenID)
	bootstrapTokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: metav1.NamespaceSystem,
		},
	}
	tokenSecret, err := generateRandomStringFromCharset(16, characterSetResourceNameFragment)
	if err != nil {
		return err
	}

	bootstrapTokenSecret.Data = map[string][]byte{
		bootstraptokenapi.BootstrapTokenIDKey:               []byte(tokenID),
		bootstraptokenapi.BootstrapTokenSecretKey:           []byte(tokenSecret),
		bootstraptokenapi.BootstrapTokenExpirationKey:       []byte(metav1.Now().Add(firewallBootstrapTokenExpiration).Format(time.RFC3339)),
		bootstraptokenapi.BootstrapTokenDescriptionKey:      []byte(fmt.Sprintf("Bootstrap token for firewall deployment %s/%s", r.Target.GetNamespace(), r.Target.GetName())),
		bootstraptokenapi.BootstrapTokenUsageAuthentication: []byte("true"),
		bootstraptokenapi.BootstrapTokenUsageSigningKey:     []byte("true"),
		bootstraptokenapi.BootstrapTokenExtraGroupsKey:      []byte(firewallBootstrapTokenAuthExtraGroups),
	}
	err = c.c.GetSeedClient().Create(r.Ctx, bootstrapTokenSecret)
	if err != nil {
		return err
	}

	return c.c.GetSeedClient().Update(r.Ctx, r.Target)
}

func generateNewTokenID() (string, error) {
	return generateRandomStringFromCharset(6, characterSetResourceNameFragment)
}

func generateRandomStringFromCharset(n int, allowedCharacters string) (string, error) {
	output := make([]byte, n)
	maximum := new(big.Int).SetInt64(int64(len(allowedCharacters)))
	for i := range output {
		randomCharacter, err := rand.Int(rand.Reader, maximum)
		if err != nil {
			return "", err
		}
		output[i] = allowedCharacters[randomCharacter.Int64()]
	}
	return string(output), nil
}
