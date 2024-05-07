package update

import (
	"context"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
	metaltestclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_controller_autoUpdateOS(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	err := v2.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		name              string
		metalMocks        *metaltestclient.MetalMockFns
		fwDeploy          *v2.FirewallDeployment
		existingFws       []v2.Firewall
		postTestFn        func(t *testing.T, c client.Client)
		want              bool
		withinMaintenance bool
		wantErr           error
	}{
		{
			name: "auto-update disabled",
			fwDeploy: &v2.FirewallDeployment{
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallTemplateSpec{
						Spec: v2.FirewallSpec{
							Image: "a",
						},
					},
					AutoUpdate: v2.FirewallAutoUpdate{
						MachineImage: false,
					},
				},
			},
			withinMaintenance: true,
			wantErr:           nil,
		},
		{
			name: "not in maintenance time window",
			fwDeploy: &v2.FirewallDeployment{
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallTemplateSpec{
						Spec: v2.FirewallSpec{
							Image: "a",
						},
					},
					AutoUpdate: v2.FirewallAutoUpdate{
						MachineImage: true,
					},
				},
			},
			withinMaintenance: false,
			wantErr:           nil,
		},
		{
			name: "not in maintenance time window",
			fwDeploy: &v2.FirewallDeployment{
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallTemplateSpec{
						Spec: v2.FirewallSpec{
							Image: "a",
						},
					},
					AutoUpdate: v2.FirewallAutoUpdate{
						MachineImage: true,
					},
				},
			},
			withinMaintenance: false,
			wantErr:           nil,
		},
		{
			name: "auto-update when using shorthand image notation",
			fwDeploy: &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "firewall",
				},
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallTemplateSpec{
						Spec: v2.FirewallSpec{
							Image: "firewall-ubuntu-3.0",
						},
					},
					AutoUpdate: v2.FirewallAutoUpdate{
						MachineImage: true,
					},
				},
			},
			withinMaintenance: true,
			metalMocks: &metaltestclient.MetalMockFns{
				Image: func(mock *mock.Mock) {
					mock.On("FindLatestImage", image.NewFindLatestImageParams().WithID("firewall-ubuntu-3.0").WithContext(ctx), nil).Return(&image.FindLatestImageOK{
						Payload: &models.V1ImageResponse{
							ID: pointer.Pointer("firewall-ubuntu-3.0.20240503"),
						},
					}, nil)
				},
			},
			existingFws: []v2.Firewall{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a",
						Namespace: "firewall",
					},
					Status: v2.FirewallStatus{
						MachineStatus: &v2.MachineStatus{
							ImageID: "firewall-ubuntu-3.0.20240101",
						},
					},
				},
			},
			postTestFn: func(t *testing.T, c client.Client) {
				fwdeploy := &v2.FirewallDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deployment",
						Namespace: "firewall",
					},
				}
				err := c.Get(context.Background(), client.ObjectKeyFromObject(fwdeploy), fwdeploy)
				require.NoError(t, err)

				assert.Equal(t, "firewall-ubuntu-3.0", fwdeploy.Spec.Template.Spec.Image)
				assert.Equal(t, fwdeploy.Annotations[v2.RollSetAnnotation], strconv.FormatBool(true))
			},
			wantErr: nil,
		},
		{
			name: "auto-update when using fully-qualified image notation",
			fwDeploy: &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deployment",
					Namespace: "firewall",
				},
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallTemplateSpec{
						Spec: v2.FirewallSpec{
							Image: "firewall-ubuntu-3.0.20230101",
						},
					},
					AutoUpdate: v2.FirewallAutoUpdate{
						MachineImage: true,
					},
				},
			},
			withinMaintenance: true,
			metalMocks: &metaltestclient.MetalMockFns{
				Image: func(mock *mock.Mock) {
					mock.On("FindLatestImage", image.NewFindLatestImageParams().WithID("firewall-ubuntu-3.0").WithContext(ctx), nil).Return(&image.FindLatestImageOK{
						Payload: &models.V1ImageResponse{
							ID: pointer.Pointer("firewall-ubuntu-3.0.20240503"),
						},
					}, nil)
				},
			},
			existingFws: []v2.Firewall{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "a",
						Namespace: "firewall",
					},
					Status: v2.FirewallStatus{
						MachineStatus: &v2.MachineStatus{
							ImageID: "firewall-ubuntu-3.0.20230101",
						},
					},
				},
			},
			postTestFn: func(t *testing.T, c client.Client) {
				fwdeploy := &v2.FirewallDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "deployment",
						Namespace: "firewall",
					},
				}
				err := c.Get(context.Background(), client.ObjectKeyFromObject(fwdeploy), fwdeploy)
				require.NoError(t, err)

				assert.Equal(t, "firewall-ubuntu-3.0.20240503", fwdeploy.Spec.Template.Spec.Image)
				assert.Equal(t, fwdeploy.Annotations[v2.RollSetAnnotation], strconv.FormatBool(true))
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mc := metaltestclient.NewMetalMockClient(t, tt.metalMocks)

			latestSet := v2.FirewallSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "firewall",
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(tt.fwDeploy, v2.GroupVersion.WithKind("FirewallDeployment")),
					},
				},
			}

			var ownedFirewalls []v2.Firewall
			for _, existingFw := range tt.existingFws {
				ownedFirewall := existingFw.DeepCopy()
				ownedFirewall.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(&latestSet, v2.GroupVersion.WithKind("FirewallSet")),
				}
				ownedFirewalls = append(ownedFirewalls, *ownedFirewall)
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithLists(
				&v2.FirewallSetList{Items: []v2.FirewallSet{latestSet}},
				&v2.FirewallList{Items: ownedFirewalls},
			).WithObjects(tt.fwDeploy).Build()

			cc, err := config.New(&config.NewControllerConfig{
				SeedClient:     client,
				SkipValidation: true,
			})
			require.NoError(t, err)

			c := &controller{
				c:          cc,
				imageCache: newImageCache(mc),
			}

			r := &controllers.Ctx[*v2.FirewallDeployment]{
				Ctx:               ctx,
				Log:               logr.Logger{},
				Target:            tt.fwDeploy,
				WithinMaintenance: tt.withinMaintenance,
			}

			err = c.autoUpdateOS(r)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			if tt.postTestFn != nil {
				tt.postTestFn(t, client)
			}
		})
	}
}
