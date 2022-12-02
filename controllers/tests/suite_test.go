package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-logr/zapr"
	"github.com/go-openapi/strfmt"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/net"
	"github.com/metal-stack/metal-lib/pkg/tag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	//+kubebuilder:scaffold:imports
)

var ctx context.Context
var cancel context.CancelFunc
var k8sClient client.Client
var testEnv *envtest.Environment
var t *testing.T
var metalClient metalgo.Client

func TestAPIs(testing *testing.T) {
	RegisterFailHandler(Fail)

	t = testing

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	zcfg := zap.NewProductionConfig()
	zcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	zcfg.EncoderConfig.TimeKey = "timestamp"
	zcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	l, err := zcfg.Build()
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(zapr.NewLogger(l))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crds")},
		ErrorIfCRDPathMissing: true,
		// AttachControlPlaneOutput: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhooks")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		CertDir:            testEnv.WebhookInstallOptions.LocalServingCertDir,
		Host:               testEnv.WebhookInstallOptions.LocalServingHost,
		Port:               testEnv.WebhookInstallOptions.LocalServingPort,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&deployment.Config{
		ControllerConfig: deployment.ControllerConfig{
			Seed:          k8sClient,
			Metal:         metalClient,
			Namespace:     "test",
			ClusterID:     "cluster-a",
			ClusterTag:    fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			ClusterAPIURL: "http://shoot-api",
			K8sVersion:    semver.MustParse("v1.25.0"),
			Recorder:      mgr.GetEventRecorderFor("firewall-deployment-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("deployment"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  k8sClient,
			Metal:                 metalClient,
			Namespace:             "test",
			ClusterID:             "cluster-a",
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			FirewallHealthTimeout: 20 * time.Minute,
			Recorder:              mgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:           k8sClient,
			Shoot:          k8sClient,
			Metal:          metalClient,
			Namespace:      "test",
			ShootNamespace: v2.FirewallShootNamespace,
			ClusterID:      "cluster-a",
			ClusterTag:     fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			Recorder:       mgr.GetEventRecorderFor("firewall-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&monitor.Config{
		ControllerConfig: monitor.ControllerConfig{
			Seed:          k8sClient,
			Shoot:         k8sClient,
			Namespace:     v2.FirewallShootNamespace,
			SeedNamespace: "test",
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall-monitor"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:scheme
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var (
	testTime  = time.Now()
	firewall1 = &models.V1FirewallResponse{
		Allocation: &models.V1MachineAllocation{
			BootInfo: &models.V1BootInfo{
				Bootloaderid: pointer.Pointer("bootloaderid"),
				Cmdline:      pointer.Pointer("cmdline"),
				ImageID:      pointer.Pointer("imageid"),
				Initrd:       pointer.Pointer("initrd"),
				Kernel:       pointer.Pointer("kernel"),
				OsPartition:  pointer.Pointer("ospartition"),
				PrimaryDisk:  pointer.Pointer("primarydisk"),
			},
			Created:          pointer.Pointer(strfmt.DateTime(testTime.Add(-14 * 24 * time.Hour))),
			Creator:          pointer.Pointer("creator"),
			Description:      "firewall allocation 1",
			Filesystemlayout: fsl1,
			Hostname:         pointer.Pointer("firewall-hostname-1"),
			Image:            image1,
			Name:             pointer.Pointer("firewall-1"),
			Networks: []*models.V1MachineNetwork{
				{
					Asn:                 pointer.Pointer(int64(200)),
					Destinationprefixes: []string{"2.2.2.2"},
					Ips:                 []string{"1.1.1.1"},
					Nat:                 pointer.Pointer(false),
					Networkid:           pointer.Pointer("private"),
					Networktype:         pointer.Pointer(net.PrivatePrimaryUnshared),
					Prefixes:            []string{"prefixes"},
					Private:             pointer.Pointer(true),
					Underlay:            pointer.Pointer(false),
					Vrf:                 pointer.Pointer(int64(100)),
				},
			},
			Project:    pointer.Pointer("project-1"),
			Reinstall:  pointer.Pointer(false),
			Role:       pointer.Pointer(models.V1MachineAllocationRoleFirewall),
			SSHPubKeys: []string{"sshpubkey"},
			Succeeded:  pointer.Pointer(true),
			UserData:   "---userdata---",
		},
		Bios: &models.V1MachineBIOS{
			Date:    pointer.Pointer("biosdata"),
			Vendor:  pointer.Pointer("biosvendor"),
			Version: pointer.Pointer("biosversion"),
		},
		Description: "firewall 1",
		Events: &models.V1MachineRecentProvisioningEvents{
			CrashLoop:            pointer.Pointer(false),
			FailedMachineReclaim: pointer.Pointer(false),
			LastErrorEvent: &models.V1MachineProvisioningEvent{
				Event:   pointer.Pointer("Crashed"),
				Message: "crash",
				Time:    strfmt.DateTime(testTime.Add(-10 * 24 * time.Hour)),
			},
			LastEventTime: strfmt.DateTime(testTime.Add(-7 * 24 * time.Hour)),
			Log: []*models.V1MachineProvisioningEvent{
				{
					Event:   pointer.Pointer("Phoned Home"),
					Message: "phoning home",
					Time:    strfmt.DateTime(testTime.Add(-7 * 24 * time.Hour)),
				},
			},
		},
		Hardware: &models.V1MachineHardware{
			CPUCores: pointer.Pointer(int32(16)),
			Disks:    []*models.V1MachineBlockDevice{},
			Memory:   pointer.Pointer(int64(32)),
			Nics:     []*models.V1MachineNic{},
		},
		ID: pointer.Pointer("1"),
		Ledstate: &models.V1ChassisIdentifyLEDState{
			Description: pointer.Pointer(""),
			Value:       pointer.Pointer(""),
		},
		Liveliness: pointer.Pointer("Alive"),
		Name:       "firewall-1",
		Partition:  partition1,
		Rackid:     "rack-1",
		Size:       size1,
		State: &models.V1MachineState{
			Description:        pointer.Pointer("state"),
			Issuer:             "issuer",
			MetalHammerVersion: pointer.Pointer("version"),
			Value:              pointer.Pointer(""),
		},
		Tags: []string{"a"},
	}
	fsl1 = &models.V1FilesystemLayoutResponse{
		Constraints: &models.V1FilesystemLayoutConstraints{
			Images: map[string]string{
				"os-image": "*",
			},
			Sizes: []string{"size1"},
		},
		Description: "fsl 1",
		Disks: []*models.V1Disk{
			{
				Device: pointer.Pointer("/dev/sda"),
				Partitions: []*models.V1DiskPartition{
					{
						Gpttype: pointer.Pointer("ef00"),
						Label:   "efi",
						Number:  pointer.Pointer(int64(1)),
						Size:    pointer.Pointer(int64(1000)),
					},
				},
				Wipeonreinstall: pointer.Pointer(true),
			},
		},
		Filesystems: []*models.V1Filesystem{
			{
				Createoptions: []string{"-F 32"},
				Device:        pointer.Pointer("/dev/sda1"),
				Format:        pointer.Pointer("vfat"),
				Label:         "efi",
				Mountoptions:  []string{"noexec"},
				Path:          "/boot/efi",
			},
			{
				Createoptions: []string{},
				Device:        pointer.Pointer("tmpfs"),
				Format:        pointer.Pointer("tmpfs"),
				Label:         "",
				Mountoptions:  []string{"noexec"},
				Path:          "/tmp",
			},
		},
		ID: pointer.Pointer("1"),
		Logicalvolumes: []*models.V1LogicalVolume{
			{
				Lvmtype:     pointer.Pointer("linear"),
				Name:        pointer.Pointer("varlib"),
				Size:        pointer.Pointer(int64(5000)),
				Volumegroup: pointer.Pointer("lvm"),
			},
		},
		Name: "fsl1",
		Raid: []*models.V1Raid{},
		Volumegroups: []*models.V1VolumeGroup{
			{
				Devices: []string{"/dev/nvme0n1"},
				Name:    pointer.Pointer("lvm"),
				Tags:    []string{},
			},
		},
	}
	imageExpiration = pointer.Pointer(strfmt.DateTime(testTime.Add(3 * 24 * time.Hour)))
	image1          = &models.V1ImageResponse{
		Classification: "supported",
		Description:    "debian-description",
		ExpirationDate: imageExpiration,
		Features:       []string{"machine"},
		ID:             pointer.Pointer("debian"),
		Name:           "debian-name",
		URL:            "debian-url",
		Usedby:         []string{"456"},
	}
	partition1 = &models.V1PartitionResponse{
		Bootconfig: &models.V1PartitionBootConfiguration{
			Commandline: "commandline",
			Imageurl:    "imageurl",
			Kernelurl:   "kernelurl",
		},
		Description:                "partition 1",
		ID:                         pointer.Pointer("1"),
		Mgmtserviceaddress:         "mgmt",
		Name:                       "partition-1",
		Privatenetworkprefixlength: 24,
	}
	size1 = &models.V1SizeResponse{
		Constraints: []*models.V1SizeConstraint{
			{
				Max:  pointer.Pointer(int64(2)),
				Min:  pointer.Pointer(int64(1)),
				Type: pointer.Pointer("storage"),
			},
			{
				Max:  pointer.Pointer(int64(4)),
				Min:  pointer.Pointer(int64(3)),
				Type: pointer.Pointer("memory"),
			},
			{
				Max:  pointer.Pointer(int64(6)),
				Min:  pointer.Pointer(int64(5)),
				Type: pointer.Pointer("cores"),
			},
		},
		Description: "size 1",
		ID:          pointer.Pointer("1"),
		Name:        "size-1",
	}
	network1 = &models.V1NetworkResponse{
		Description:         "network 1",
		Destinationprefixes: []string{"dest"},
		ID:                  pointer.Pointer("nw1"),
		Labels:              map[string]string{"a": "b"},
		Name:                "network-1",
		Nat:                 pointer.Pointer(true),
		Parentnetworkid:     "",
		Partitionid:         "partition-1",
		Prefixes:            []string{"prefix"},
		Privatesuper:        pointer.Pointer(true),
		Projectid:           "",
		Shared:              false,
		Underlay:            pointer.Pointer(true),
		Usage: &models.V1NetworkUsage{
			AvailableIps:      pointer.Pointer(int64(100)),
			AvailablePrefixes: pointer.Pointer(int64(200)),
			UsedIps:           pointer.Pointer(int64(300)),
			UsedPrefixes:      pointer.Pointer(int64(400)),
		},
		Vrf:       50,
		Vrfshared: true,
	}
)
