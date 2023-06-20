package helper

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetSSHPublicKey(ctx context.Context, seedClient client.Client, secretName, namespace string) (string, error) {
	sshSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
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
