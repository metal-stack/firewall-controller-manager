package timeout

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	controllerconfig "github.com/metal-stack/firewall-controller-manager/api/v2/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTimeoutController_deleteIfUnhealthyOrTimeout(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		createTimeout time.Duration
		healthTimeout time.Duration
		firewall      func(now time.Time, name string) *v2.Firewall
		wantDeleted   bool
		wantRequeue   bool
	}{
		{
			name:          "deletes firewall after health timeout",
			healthTimeout: 5 * time.Minute,
			firewall: func(now time.Time, name string) *v2.Firewall {
				// Seed connection has been false for 10 minutes => timeout exceeded.
				return newFirewall(name, now.Add(-10*time.Minute), v2.ConditionFalse)
			},
			wantDeleted: true,
		},
		{
			name:          "requeues before health timeout",
			healthTimeout: 5 * time.Minute,
			firewall: func(now time.Time, name string) *v2.Firewall {
				// Seed connection has been false for 4 minutes => not timed out yet.
				return newFirewall(name, now.Add(-4*time.Minute), v2.ConditionFalse)
			},
			wantRequeue: true,
		},
		{
			name:          "deletes firewall after create timeout",
			createTimeout: 5 * time.Minute,
			firewall: func(now time.Time, name string) *v2.Firewall {
				return newCreatingFirewall(name, now.Add(-10*time.Minute), v2.ConditionFalse)
			},
			wantDeleted: true,
		},
		{
			name:          "returns zero result for healthy firewall",
			healthTimeout: 5 * time.Minute,
			firewall: func(now time.Time, name string) *v2.Firewall {
				return newFirewall(name, now.Add(-1*time.Minute), v2.ConditionTrue)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fw := tt.firewall(now, fmt.Sprintf("fw-%s", t.Name()))
			c := newTestController(t, tt.createTimeout, tt.healthTimeout, fw)

			res, err := c.deleteIfUnhealthyOrTimeout(context.Background(), fw)
			if err != nil {
				t.Fatalf("deleteIfUnhealthyOrTimeout() error = %v", err)
			}

			err = c.client.Get(context.Background(), client.ObjectKeyFromObject(fw), &v2.Firewall{})
			if tt.wantDeleted {
				if !apierrors.IsNotFound(err) {
					t.Fatalf("expected firewall to be deleted, got err = %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected firewall to still exist, got err = %v", err)
			}
			if tt.wantRequeue && res.RequeueAfter <= 0 {
				t.Fatalf("expected positive requeue duration, got %s", res.RequeueAfter)
			}
			if !tt.wantRequeue && res.RequeueAfter != 0 {
				t.Fatalf("expected zero requeue duration, got %s", res.RequeueAfter)
			}
		})
	}
}

func newFirewall(name string, seedConnectedTransition time.Time, seedConnectedStatus v2.ConditionStatus) *v2.Firewall {
	baseTs := seedConnectedTransition
	if baseTs.After(time.Now().Add(-10 * time.Minute)) {
		baseTs = time.Now().Add(-10 * time.Minute)
	}

	return &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Status: v2.FirewallStatus{
			Phase: v2.FirewallPhaseRunning,
			Conditions: v2.Conditions{
				cond(v2.FirewallCreated, v2.ConditionTrue, baseTs),
				cond(v2.FirewallReady, v2.ConditionTrue, baseTs),
				cond(v2.FirewallProvisioned, v2.ConditionTrue, baseTs),
				cond(v2.FirewallControllerConnected, v2.ConditionTrue, baseTs),
				cond(v2.FirewallDistanceConfigured, v2.ConditionTrue, baseTs),
				cond(v2.FirewallControllerSeedConnected, seedConnectedStatus, seedConnectedTransition),
			},
		},
	}
}

func newCreatingFirewall(name string, readyTransition time.Time, readyStatus v2.ConditionStatus) *v2.Firewall {
	baseTs := readyTransition
	if baseTs.After(time.Now().Add(-10 * time.Minute)) {
		baseTs = time.Now().Add(-10 * time.Minute)
	}

	return &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Status: v2.FirewallStatus{
			Phase: v2.FirewallPhaseCreating,
			Conditions: v2.Conditions{
				cond(v2.FirewallCreated, v2.ConditionTrue, baseTs),
				cond(v2.FirewallReady, readyStatus, readyTransition),
				cond(v2.FirewallProvisioned, v2.ConditionFalse, readyTransition),
			},
		},
	}
}

func newTestController(t *testing.T, createTimeout, healthTimeout time.Duration, objs ...client.Object) *controller {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(v2.AddToScheme(scheme))

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	cfg, err := controllerconfig.New(&controllerconfig.NewControllerConfig{
		SeedClient:            cl,
		CreateTimeout:         createTimeout,
		FirewallHealthTimeout: healthTimeout,
		SkipValidation:        true,
	})
	if err != nil {
		t.Fatalf("unable to create controller config: %v", err)
	}

	return &controller{
		c:        cfg,
		client:   cl,
		log:      logr.Discard(),
		recorder: events.NewFakeRecorder(1),
	}
}

func cond(t v2.ConditionType, status v2.ConditionStatus, ts time.Time) v2.Condition {
	return v2.Condition{
		Type:               t,
		Status:             status,
		LastTransitionTime: metav1.NewTime(ts),
		LastUpdateTime:     metav1.NewTime(ts),
	}
}
