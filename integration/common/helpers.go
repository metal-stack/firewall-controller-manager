package testcommon

import (
	"context"
	"sort"
	"time"

	. "github.com/onsi/gomega"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	interval = 250 * time.Millisecond
)

func WaitForCondition[O client.Object](c client.Client, ctx context.Context, of O, getter func(O) v2.Conditions, t v2.ConditionType, s v2.ConditionStatus, timeout time.Duration) *v2.Condition {
	var cond *v2.Condition
	Eventually(func() v2.ConditionStatus {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(of), of)).To(Succeed())
		cond = getter(of).Get(t)
		return cond.Status
	}, timeout, interval).Should(Equal(s), "waiting for condition %q to reach status %q", t, s)
	return cond
}

// waitForResourceAmount waits for the given amount of resources and returns the newest one
func WaitForResourceAmount[O client.Object, L client.ObjectList](c client.Client, ctx context.Context, namespace string, amount int, list L, getter func(L) []O, timeout time.Duration) O {
	var items []O
	Eventually(func() []O {
		err := c.List(ctx, list, client.InNamespace(namespace))
		Expect(err).To(Not(HaveOccurred()))
		items = getter(list)
		return items
	}, timeout, interval).Should(HaveLen(amount))

	if amount == 0 {
		var o O
		return o
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].GetCreationTimestamp().After(items[j].GetCreationTimestamp().Time)
	})

	return items[0]
}
