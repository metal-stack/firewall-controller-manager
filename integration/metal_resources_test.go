package controllers_test

import (
	"context"
	"time"

	apiv2client "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	"github.com/metal-stack/firewall-controller-manager/internal/test"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	metalClient apiv2client.Client
	metalMock   *test.Client

	testTime  = time.Now()
	firewall1 = &apiv2.Machine{
		Allocation: &apiv2.MachineAllocation{
			Meta: &apiv2.Meta{
				CreatedAt: timestamppb.New(testTime.Add(-14 * 24 * time.Hour)),
			},
			CreatedBy:        "creator",
			Description:      "firewall allocation 1",
			FilesystemLayout: fsl1,
			Hostname:         "firewall-hostname-1",
			Image:            image1,
			Name:             "firewall-1",
			Networks: []*apiv2.MachineNetwork{
				{
					Asn:                 uint32(200),
					DestinationPrefixes: []string{"2.2.2.2"},
					Ips:                 []string{"1.1.1.1"},
					NatType:             apiv2.NATType_NAT_TYPE_NONE,
					Network:             "private",
					NetworkType:         apiv2.NetworkType_NETWORK_TYPE_CHILD,
					Prefixes:            []string{"prefixes"},
					Vrf:                 uint64(100),
				},
			},
			Project:        "project-1",
			AllocationType: apiv2.MachineAllocationType_MACHINE_ALLOCATION_TYPE_FIREWALL,
			SshPublicKeys:  []string{"sshpubkey"},
			Userdata:       "---userdata---",
		},
		RecentProvisioningEvents: &apiv2.MachineRecentProvisioningEvents{
			Events: []*apiv2.MachineProvisioningEvent{
				{
					Time:    timestamppb.New(testTime.Add(-7 * 24 * time.Hour)),
					Event:   apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_PHONED_HOME,
					Message: "phoning home",
				},
			},
			LastErrorEvent: &apiv2.MachineProvisioningEvent{
				Time:    timestamppb.New(testTime.Add(-10 * 24 * time.Hour)),
				Event:   apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_CRASHED,
				Message: "crash",
			},
			LastEventTime: timestamppb.New(testTime.Add(-7 * 24 * time.Hour)),
			State:         apiv2.MachineProvisioningEventState_MACHINE_PROVISIONING_EVENT_STATE_UNSPECIFIED,
		},
		Hardware: &apiv2.MachineHardware{
			Cpus: []*apiv2.MetalCPU{
				{
					Cores: 16,
				},
			},
			Disks:  []*apiv2.MachineBlockDevice{},
			Memory: uint64(32),
			Nics:   []*apiv2.MachineNic{},
		},
		Uuid: "1",
		Status: &apiv2.MachineStatus{
			Condition: &apiv2.MachineCondition{
				Description: "state",
				Issuer:      "issuer",
			},
			LedState:           &apiv2.MachineChassisIdentifyLEDState{},
			Liveliness:         apiv2.MachineLiveliness_MACHINE_LIVELINESS_ALIVE,
			MetalHammerVersion: "version",
		},
		Partition: partition1,
		Rack:      "rack-1",
		Size:      size1,
	}
	firewall2 = func(event, message string) *apiv2.Machine {
		return &apiv2.Machine{
			Allocation: &apiv2.MachineAllocation{
				Meta: &apiv2.Meta{
					CreatedAt: timestamppb.New(testTime),
				},
				CreatedBy:        "creator",
				Description:      "firewall allocation 2",
				FilesystemLayout: fsl1,
				Hostname:         "firewall-hostname-2",
				Image:            image1,
				Name:             "firewall-2",
				Networks: []*apiv2.MachineNetwork{
					{
						Asn:                 uint32(200),
						DestinationPrefixes: []string{"2.2.2.2"},
						Ips:                 []string{"1.1.1.1"},
						NatType:             apiv2.NATType_NAT_TYPE_NONE,
						Network:             "private",
						NetworkType:         apiv2.NetworkType_NETWORK_TYPE_CHILD,
						Prefixes:            []string{"prefixes"},
						Vrf:                 uint64(100),
					},
				},
				Project:        "project-1",
				AllocationType: apiv2.MachineAllocationType_MACHINE_ALLOCATION_TYPE_FIREWALL,
				SshPublicKeys:  []string{"sshpubkey"},
				Userdata:       "---userdata---",
			},
			RecentProvisioningEvents: &apiv2.MachineRecentProvisioningEvents{
				Events: []*apiv2.MachineProvisioningEvent{
					{
						Time:    timestamppb.New(testTime.Add(-7 * 24 * time.Hour)),
						Event:   provisioningEventType(event),
						Message: message,
					},
				},
				LastErrorEvent: &apiv2.MachineProvisioningEvent{},
				LastEventTime:  timestamppb.New(testTime.Add(-1 * time.Minute)),
				State:          apiv2.MachineProvisioningEventState_MACHINE_PROVISIONING_EVENT_STATE_UNSPECIFIED,
			},
			Hardware: &apiv2.MachineHardware{
				Cpus: []*apiv2.MetalCPU{
					{
						Cores: 16,
					},
				},
				Disks:  []*apiv2.MachineBlockDevice{},
				Memory: uint64(32),
				Nics:   []*apiv2.MachineNic{},
			},
			Uuid: "2",
			Status: &apiv2.MachineStatus{
				Condition: &apiv2.MachineCondition{
					Description: "state",
					Issuer:      "issuer",
				},
				LedState:           &apiv2.MachineChassisIdentifyLEDState{},
				Liveliness:         apiv2.MachineLiveliness_MACHINE_LIVELINESS_ALIVE,
				MetalHammerVersion: "version",
			},
			Partition: partition1,
			Rack:      "rack-1",
			Size:      size1,
		}
	}
	fsl1 = &apiv2.FilesystemLayout{
		Constraints: &apiv2.FilesystemLayoutConstraints{
			Images: map[string]string{
				"os-image": "*",
			},
			Sizes: []string{"size1"},
		},
		Description: new("fsl 1"),
		Disks: []*apiv2.Disk{
			{
				Device: "/dev/sda",
				Partitions: []*apiv2.DiskPartition{
					{
						GptType: apiv2.GPTType_GPT_TYPE_BOOT.Enum(),
						Label:   new("efi"),
						Number:  uint32(1),
						Size:    uint64(1000),
					},
				},
			},
		},
		Filesystems: []*apiv2.Filesystem{
			{
				CreateOptions: []string{"-F 32"},
				Device:        "/dev/sda1",
				Format:        apiv2.Format_FORMAT_VFAT,
				Label:         new("efi"),
				MountOptions:  []string{"noexec"},
				Path:          new("/boot/efi"),
			},
			{
				CreateOptions: []string{},
				Device:        "tmpfs",
				Format:        apiv2.Format_FORMAT_TMPFS,
				Label:         new(""),
				MountOptions:  []string{"noexec"},
				Path:          new("/tmp"),
			},
		},
		Id: "1",
		LogicalVolumes: []*apiv2.LogicalVolume{
			{
				LvmType:     apiv2.LVMType_LVM_TYPE_LINEAR,
				Name:        "varlib",
				Size:        uint64(5000),
				VolumeGroup: "lvm",
			},
		},
		Name: new("fsl1"),
		Raid: []*apiv2.Raid{},
		VolumeGroups: []*apiv2.VolumeGroup{
			{
				Devices: []string{"/dev/nvme0n1"},
				Name:    "lvm",
				Tags:    []string{},
			},
		},
	}
	imageExpiration = timestamppb.New(testTime.Add(3 * 24 * time.Hour))
	image1          = &apiv2.Image{
		Classification: apiv2.ImageClassification_IMAGE_CLASSIFICATION_SUPPORTED,
		Description:    new("firewall-image-description"),
		ExpiresAt:      imageExpiration,
		Features:       []apiv2.ImageFeature{apiv2.ImageFeature_IMAGE_FEATURE_FIREWALL},
		Id:             "firewall-ubuntu-2.0",
		Name:           new("firewall-image-name"),
		Url:            "firewall-image-url",
	}
	partition1 = &apiv2.Partition{
		BootConfiguration: &apiv2.PartitionBootConfiguration{
			Commandline: "commandline",
			ImageUrl:    "imageurl",
			KernelUrl:   "kernelurl",
		},
		Description:          "partition 1",
		Id:                   "1",
		MgmtServiceAddresses: []string{"mgmt"},
	}
	size1 = &apiv2.Size{
		Constraints: []*apiv2.SizeConstraint{
			{
				Max:  uint64(2),
				Min:  uint64(1),
				Type: apiv2.SizeConstraintType_SIZE_CONSTRAINT_TYPE_STORAGE,
			},
			{
				Max:  uint64(4),
				Min:  uint64(3),
				Type: apiv2.SizeConstraintType_SIZE_CONSTRAINT_TYPE_MEMORY,
			},
			{
				Max:  uint64(6),
				Min:  uint64(5),
				Type: apiv2.SizeConstraintType_SIZE_CONSTRAINT_TYPE_CORES,
			},
		},
		Description: new("size 1"),
		Id:          "1",
		Name:        new("size-1"),
	}
	network1 = &apiv2.Network{
		Description:         new("network 1"),
		DestinationPrefixes: []string{"dest"},
		Id:                  "nw1",
		Name:                new("network-1"),
		NatType:             apiv2.NATType_NAT_TYPE_IPV4_MASQUERADE,
		Partition:           new("partition-1"),
		Prefixes:            []string{"prefix"},
		Project:             new(""),
		Type:                apiv2.NetworkType_NETWORK_TYPE_UNDERLAY,
		Vrf:                 new(uint32(50)),
	}
)

func provisioningEventType(event string) apiv2.MachineProvisioningEventType {
	switch event {
	case "Phoned Home":
		return apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_PHONED_HOME
	case "Installing":
		return apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_INSTALLING
	case "Crashed":
		return apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_CRASHED
	default:
		return apiv2.MachineProvisioningEventType_MACHINE_PROVISIONING_EVENT_TYPE_UNSPECIFIED
	}
}

// we are sharing a metal client for the tests, so we need to make sure we do not run contradicting tests in parallel
// the shared metal mock client is reconfigured with this function
func swapMetalClient(fns func(*test.Client)) {
	if metalMock != nil && fns != nil {
		fns(metalMock)
	}
}

// mockMachine configures the machine service handlers to return the given firewalls.
func mockMachine(m *test.Client, fws ...*apiv2.Machine) {
	machine := fws[0]

	m.OnMachineGet = func(_ context.Context, _ *apiv2.MachineServiceGetRequest) (*apiv2.MachineServiceGetResponse, error) {
		return &apiv2.MachineServiceGetResponse{Machine: machine}, nil
	}
	m.OnMachineCreate = func(_ context.Context, _ *apiv2.MachineServiceCreateRequest) (*apiv2.MachineServiceCreateResponse, error) {
		return &apiv2.MachineServiceCreateResponse{Machine: machine}, nil
	}
	m.OnMachineList = func(_ context.Context, _ *apiv2.MachineServiceListRequest) (*apiv2.MachineServiceListResponse, error) {
		return &apiv2.MachineServiceListResponse{Machines: fws}, nil
	}
}

// mockMachineDelete configures the machine delete handler to return the given firewall.
func mockMachineDelete(m *test.Client, machine *apiv2.Machine) {
	m.OnMachineDelete = func(_ context.Context, _ *apiv2.MachineServiceDeleteRequest) (*apiv2.MachineServiceDeleteResponse, error) {
		return &apiv2.MachineServiceDeleteResponse{Machine: machine}, nil
	}
}

// mockMachineDeleteErr configures the machine delete handler to return the given error.
func mockMachineDeleteErr(m *test.Client, err error) {
	m.OnMachineDelete = func(_ context.Context, _ *apiv2.MachineServiceDeleteRequest) (*apiv2.MachineServiceDeleteResponse, error) {
		return nil, err
	}
}

// mockMachineUpdate configures the machine update handler to succeed.
func mockMachineUpdate(m *test.Client) {
	m.OnMachineUpdate = func(_ context.Context, _ *apiv2.MachineServiceUpdateRequest) (*apiv2.MachineServiceUpdateResponse, error) {
		return &apiv2.MachineServiceUpdateResponse{Machine: &apiv2.Machine{}}, nil
	}
}

// mockNetwork configures the network service handler to return the given network.
func mockNetwork(m *test.Client, nw *apiv2.Network) {
	m.OnNetworkGet = func(_ context.Context, _ *apiv2.NetworkServiceGetRequest) (*apiv2.NetworkServiceGetResponse, error) {
		return &apiv2.NetworkServiceGetResponse{Network: nw}, nil
	}
}

// mockImage configures the image service handler to return the given image.
func mockImage(m *test.Client, img *apiv2.Image) {
	m.OnImageLatest = func(_ context.Context, _ *apiv2.ImageServiceLatestRequest) (*apiv2.ImageServiceLatestResponse, error) {
		return &apiv2.ImageServiceLatestResponse{Image: img}, nil
	}
}
