package deployment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
	metaltestclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/cache"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_controller_osImageHasChanged(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	err := v2.AddToScheme(scheme)
	require.NoError(t, err)

	latestSet := &v2.FirewallSet{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "firewall",
		},
	}

	tests := []struct {
		name              string
		newS              *v2.FirewallSpec
		oldS              *v2.FirewallSpec
		mocks             *metaltestclient.MetalMockFns
		existingFws       []v2.Firewall
		want              bool
		withinMaintenance bool
		wantErr           error
	}{
		{
			name: "image was updated",
			oldS: &v2.FirewallSpec{
				Image: "a",
			},
			newS: &v2.FirewallSpec{
				Image: "b",
			},
			withinMaintenance: false,
			want:              true,
			wantErr:           nil,
		},
		{
			name: "image was updated in maintenance",
			oldS: &v2.FirewallSpec{
				Image: "a",
			},
			newS: &v2.FirewallSpec{
				Image: "b",
			},
			withinMaintenance: true,
			want:              true,
			wantErr:           nil,
		},
		{
			name: "image might auto-update but not in maintenance mode",
			oldS: &v2.FirewallSpec{
				Image: "a",
			},
			newS: &v2.FirewallSpec{
				Image: "a",
			},
			withinMaintenance: false,
			want:              false,
			wantErr:           nil,
		},
		{
			name: "no auto-update because no shorthand image used",
			oldS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0.20240503",
			},
			newS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0.20240503",
			},
			mocks: &metaltestclient.MetalMockFns{
				Image: func(mock *mock.Mock) {
					mock.On("FindLatestImage", image.NewFindLatestImageParams().WithID("firewall-ubuntu-3.0.20240503").WithContext(ctx), nil).Return(&image.FindLatestImageOK{
						Payload: &models.V1ImageResponse{
							ID: pointer.Pointer("firewall-ubuntu-3.0.20240503"),
						},
					}, nil)
				},
			},
			withinMaintenance: true,
			want:              false,
			wantErr:           nil,
		},
		{
			name: "no auto-update because firewall already runs latest image",
			oldS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0",
			},
			newS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0",
			},
			mocks: &metaltestclient.MetalMockFns{
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
					ObjectMeta: v1.ObjectMeta{
						Name:      "a",
						Namespace: "firewall",
						OwnerReferences: []v1.OwnerReference{
							*v1.NewControllerRef(latestSet, v2.GroupVersion.WithKind("FirewallSet")),
						},
					},
					Status: v2.FirewallStatus{
						MachineStatus: &v2.MachineStatus{
							ImageID: "firewall-ubuntu-3.0.20240503",
						},
					},
				},
			},
			withinMaintenance: true,
			want:              false,
			wantErr:           nil,
		},
		{
			name: "auto-update because firewall not running latest image",
			oldS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0",
			},
			newS: &v2.FirewallSpec{
				Image: "firewall-ubuntu-3.0",
			},
			mocks: &metaltestclient.MetalMockFns{
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
					ObjectMeta: v1.ObjectMeta{
						Name:      "a",
						Namespace: "firewall",
						OwnerReferences: []v1.OwnerReference{
							*v1.NewControllerRef(latestSet, v2.GroupVersion.WithKind("FirewallSet")),
						},
					},
					Status: v2.FirewallStatus{
						MachineStatus: &v2.MachineStatus{
							ImageID: "firewall-ubuntu-3.0.20240501",
						},
					},
				},
			},
			withinMaintenance: true,
			want:              true,
			wantErr:           nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mc := metaltestclient.NewMetalMockClient(t, tt.mocks)

			cc, err := config.New(&config.NewControllerConfig{
				SeedClient:     fake.NewClientBuilder().WithScheme(scheme).WithLists(&v2.FirewallList{Items: tt.existingFws}).Build(),
				SkipValidation: true,
			})
			require.NoError(t, err)

			c := &controller{
				c: cc,
				imageCache: cache.New(5*time.Minute, func(ctx context.Context, id string) (*models.V1ImageResponse, error) {
					resp, err := mc.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(id).WithContext(ctx), nil)
					if err != nil {
						return nil, fmt.Errorf("latest firewall image %q not found: %w", id, err)
					}

					return resp.Payload, nil
				}),
			}

			r := &controllers.Ctx[*v2.FirewallDeployment]{
				Ctx:               ctx,
				Log:               logr.Logger{},
				Target:            &v2.FirewallDeployment{},
				WithinMaintenance: tt.withinMaintenance,
			}

			got, err := c.osImageHasChanged(r, latestSet, tt.newS, tt.oldS)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
