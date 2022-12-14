package defaults

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flatcar/container-linux-config-transpiler/config/types"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	FirewallControllerName = "firewall-controller"
	DroptailerClientName   = "droptailer"
)

func createUserdata(ctx context.Context, c client.Client, k8sVersion *semver.Version, namespace, apiServerURL string) (string, error) {
	var (
		ca    []byte
		token string
	)

	if helper.VersionGreaterOrEqual125(k8sVersion) {
		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: namespace,
			},
		}
		err := c.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return "", err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	} else {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: namespace,
			},
		}
		err := c.Get(ctx, client.ObjectKeyFromObject(sa), sa, &client.GetOptions{})
		if err != nil {
			return "", err
		}

		if len(sa.Secrets) == 0 {
			return "", fmt.Errorf("service account %q contains no valid token secret", sa.Name)
		}

		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sa.Secrets[0].Name,
				Namespace: namespace,
			},
		}
		err = c.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return "", err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	}

	if token == "" {
		return "", fmt.Errorf("no token was created")
	}

	config := &configv1.Config{
		CurrentContext: namespace,
		Clusters: []configv1.NamedCluster{
			{
				Name: namespace,
				Cluster: configv1.Cluster{
					CertificateAuthorityData: ca,
					Server:                   apiServerURL,
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: namespace,
				Context: configv1.Context{
					Cluster:  namespace,
					AuthInfo: namespace,
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name: namespace,
				AuthInfo: configv1.AuthInfo{
					Token: token,
				},
			},
		},
	}

	kubeconfig, err := runtime.Encode(configlatest.Codec, config)
	if err != nil {
		return "", fmt.Errorf("unable to encode kubeconfig for firewall: %w", err)
	}

	return renderUserdata(kubeconfig)
}

func renderUserdata(kubeconfig []byte) (string, error) {
	cfg := types.Config{}
	cfg.Systemd = types.Systemd{}

	enabled := true
	fcUnit := types.SystemdUnit{
		Name:    fmt.Sprintf("%s.service", FirewallControllerName),
		Enable:  enabled,
		Enabled: &enabled,
	}
	dcUnit := types.SystemdUnit{
		Name:    fmt.Sprintf("%s.service", DroptailerClientName),
		Enable:  enabled,
		Enabled: &enabled,
	}

	cfg.Systemd.Units = append(cfg.Systemd.Units, fcUnit, dcUnit)

	cfg.Storage = types.Storage{}

	mode := 0600
	id := 0
	ignitionFile := types.File{
		Path:       fmt.Sprintf("/etc/%s/.kubeconfig", FirewallControllerName),
		Filesystem: "root",
		Mode:       &mode,
		User: &types.FileUser{
			Id: &id,
		},
		Group: &types.FileGroup{
			Id: &id,
		},
		Contents: types.FileContents{
			Inline: string(kubeconfig),
		},
	}
	cfg.Storage.Files = append(cfg.Storage.Files, ignitionFile)

	outCfg, report := types.Convert(cfg, "", nil)
	if report.IsFatal() {
		return "", fmt.Errorf("could not transpile ignition config: %s", report.String())
	}

	userData, err := json.Marshal(outCfg)
	if err != nil {
		return "", err
	}

	return string(userData), nil
}
