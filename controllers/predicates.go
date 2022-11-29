package controllers

import "sigs.k8s.io/controller-runtime/pkg/client"

func SkipOtherNamespace(namespace string) func(object client.Object) bool {
	return func(object client.Object) bool {
		return object.GetNamespace() == namespace
	}
}
