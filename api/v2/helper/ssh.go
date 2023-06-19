package helper

import (
	"context"
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSSHPublicKey(ctx context.Context, seedClient client.Client, access *v2.ShootAccess) (string, error) {
	sshSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.SSHKeySecretName,
			Namespace: access.Namespace,
		},
	}

	err := seedClient.Get(ctx, client.ObjectKeyFromObject(sshSecret), sshSecret)
	if err != nil {
		return "", fmt.Errorf("ssh secret not found: %w", err)
	}

	sshPublicKey, ok := sshSecret.Data["id_rsa.pub"]
	if !ok {
		return "", fmt.Errorf("ssh secret does not contain a public key")
	}

	return string(sshPublicKey), nil
}
