package helper

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var scheme *runtime.Scheme

func MustNewFirewallScheme() *runtime.Scheme {
	if scheme != nil {
		return scheme
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v2.AddToScheme(scheme))

	return scheme
}
