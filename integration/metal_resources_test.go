package controllers_test

import (
	"time"

	"github.com/go-openapi/strfmt"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/models"
	metalclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/net"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

var (
	metalClient metalgo.Client

	testTime  = time.Now()
	firewall1 = &models.V1FirewallResponse{
		Allocation: &models.V1MachineAllocation{
			BootInfo: &models.V1BootInfo{
				Bootloaderid: new("bootloaderid"),
				Cmdline:      new("cmdline"),
				ImageID:      new("imageid"),
				Initrd:       new("initrd"),
				Kernel:       new("kernel"),
				OsPartition:  new("ospartition"),
				PrimaryDisk:  new("primarydisk"),
			},
			Created:          new(strfmt.DateTime(testTime.Add(-14 * 24 * time.Hour))),
			Creator:          new("creator"),
			Description:      "firewall allocation 1",
			Filesystemlayout: fsl1,
			Hostname:         new("firewall-hostname-1"),
			Image:            image1,
			Name:             new("firewall-1"),
			Networks: []*models.V1MachineNetwork{
				{
					Asn:                 new(int64(200)),
					Destinationprefixes: []string{"2.2.2.2"},
					Ips:                 []string{"1.1.1.1"},
					Nat:                 new(false),
					Networkid:           new("private"),
					Networktype:         pointer.Pointer(net.PrivatePrimaryUnshared),
					Prefixes:            []string{"prefixes"},
					Private:             new(true),
					Underlay:            new(false),
					Vrf:                 new(int64(100)),
				},
			},
			Project:    new("project-1"),
			Reinstall:  new(false),
			Role:       pointer.Pointer(models.V1MachineAllocationRoleFirewall),
			SSHPubKeys: []string{"sshpubkey"},
			Succeeded:  new(true),
			UserData:   "---userdata---",
		},
		Bios: &models.V1MachineBIOS{
			Date:    new("biosdata"),
			Vendor:  new("biosvendor"),
			Version: new("biosversion"),
		},
		Description: "firewall 1",
		Events: &models.V1MachineRecentProvisioningEvents{
			CrashLoop:            new(false),
			FailedMachineReclaim: new(false),
			LastErrorEvent: &models.V1MachineProvisioningEvent{
				Event:   new("Crashed"),
				Message: "crash",
				Time:    strfmt.DateTime(testTime.Add(-10 * 24 * time.Hour)),
			},
			LastEventTime: strfmt.DateTime(testTime.Add(-7 * 24 * time.Hour)),
			Log: []*models.V1MachineProvisioningEvent{
				{
					Event:   new("Phoned Home"),
					Message: "phoning home",
					Time:    strfmt.DateTime(testTime.Add(-7 * 24 * time.Hour)),
				},
			},
		},
		Hardware: &models.V1MachineHardware{
			CPUCores: new(int32(16)),
			Disks:    []*models.V1MachineBlockDevice{},
			Memory:   new(int64(32)),
			Nics:     []*models.V1MachineNic{},
		},
		ID: new("1"),
		Ledstate: &models.V1ChassisIdentifyLEDState{
			Description: new(""),
			Value:       new(""),
		},
		Liveliness: new("Alive"),
		Name:       "firewall-1",
		Partition:  partition1,
		Rackid:     "rack-1",
		Size:       size1,
		State: &models.V1MachineState{
			Description:        new("state"),
			Issuer:             "issuer",
			MetalHammerVersion: new("version"),
			Value:              new(""),
		},
		Tags: []string{"a"},
	}
	firewall2 = func(event, message string) *models.V1FirewallResponse {
		return &models.V1FirewallResponse{
			Allocation: &models.V1MachineAllocation{
				BootInfo: &models.V1BootInfo{
					Bootloaderid: new("bootloaderid"),
					Cmdline:      new("cmdline"),
					ImageID:      new("imageid"),
					Initrd:       new("initrd"),
					Kernel:       new("kernel"),
					OsPartition:  new("ospartition"),
					PrimaryDisk:  new("primarydisk"),
				},
				Created:          new(strfmt.DateTime(testTime)),
				Creator:          new("creator"),
				Description:      "firewall allocation 2",
				Filesystemlayout: fsl1,
				Hostname:         new("firewall-hostname-2"),
				Image:            image1,
				Name:             new("firewall-2"),
				Networks: []*models.V1MachineNetwork{
					{
						Asn:                 new(int64(200)),
						Destinationprefixes: []string{"2.2.2.2"},
						Ips:                 []string{"1.1.1.1"},
						Nat:                 new(false),
						Networkid:           new("private"),
						Networktype:         pointer.Pointer(net.PrivatePrimaryUnshared),
						Prefixes:            []string{"prefixes"},
						Private:             new(true),
						Underlay:            new(false),
						Vrf:                 new(int64(100)),
					},
				},
				Project:    new("project-1"),
				Reinstall:  new(false),
				Role:       pointer.Pointer(models.V1MachineAllocationRoleFirewall),
				SSHPubKeys: []string{"sshpubkey"},
				Succeeded:  new(true),
				UserData:   "---userdata---",
			},
			Bios: &models.V1MachineBIOS{
				Date:    new("biosdata"),
				Vendor:  new("biosvendor"),
				Version: new("biosversion"),
			},
			Description: "firewall 2",
			Events: &models.V1MachineRecentProvisioningEvents{
				CrashLoop:            new(false),
				FailedMachineReclaim: new(false),
				LastErrorEvent:       &models.V1MachineProvisioningEvent{},
				LastEventTime:        strfmt.DateTime(testTime.Add(-1 * time.Minute)),
				Log: []*models.V1MachineProvisioningEvent{
					{
						Event:   new(event),
						Message: message,
						Time:    strfmt.DateTime(testTime.Add(-7 * 24 * time.Hour)),
					},
				},
			},
			Hardware: &models.V1MachineHardware{
				CPUCores: new(int32(16)),
				Disks:    []*models.V1MachineBlockDevice{},
				Memory:   new(int64(32)),
				Nics:     []*models.V1MachineNic{},
			},
			ID: new("2"),
			Ledstate: &models.V1ChassisIdentifyLEDState{
				Description: new(""),
				Value:       new(""),
			},
			Liveliness: new("Alive"),
			Name:       "firewall-2",
			Partition:  partition1,
			Rackid:     "rack-1",
			Size:       size1,
			State: &models.V1MachineState{
				Description:        new("state"),
				Issuer:             "issuer",
				MetalHammerVersion: new("version"),
				Value:              new(""),
			},
			Tags: []string{"b"},
		}
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
				Device: new("/dev/sda"),
				Partitions: []*models.V1DiskPartition{
					{
						Gpttype: new("ef00"),
						Label:   "efi",
						Number:  new(int64(1)),
						Size:    new(int64(1000)),
					},
				},
				Wipeonreinstall: new(true),
			},
		},
		Filesystems: []*models.V1Filesystem{
			{
				Createoptions: []string{"-F 32"},
				Device:        new("/dev/sda1"),
				Format:        new("vfat"),
				Label:         "efi",
				Mountoptions:  []string{"noexec"},
				Path:          "/boot/efi",
			},
			{
				Createoptions: []string{},
				Device:        new("tmpfs"),
				Format:        new("tmpfs"),
				Label:         "",
				Mountoptions:  []string{"noexec"},
				Path:          "/tmp",
			},
		},
		ID: new("1"),
		Logicalvolumes: []*models.V1LogicalVolume{
			{
				Lvmtype:     new("linear"),
				Name:        new("varlib"),
				Size:        new(int64(5000)),
				Volumegroup: new("lvm"),
			},
		},
		Name: "fsl1",
		Raid: []*models.V1Raid{},
		Volumegroups: []*models.V1VolumeGroup{
			{
				Devices: []string{"/dev/nvme0n1"},
				Name:    new("lvm"),
				Tags:    []string{},
			},
		},
	}
	imageExpiration = new(strfmt.DateTime(testTime.Add(3 * 24 * time.Hour)))
	image1          = &models.V1ImageResponse{
		Classification: "supported",
		Description:    "firewall-image-description",
		ExpirationDate: imageExpiration,
		Features:       []string{"firewall"},
		ID:             new("firewall-ubuntu-2.0"),
		Name:           "firewall-image-name",
		URL:            "firewall-image-url",
	}
	partition1 = &models.V1PartitionResponse{
		Bootconfig: &models.V1PartitionBootConfiguration{
			Commandline: "commandline",
			Imageurl:    "imageurl",
			Kernelurl:   "kernelurl",
		},
		Description:        "partition 1",
		ID:                 new("1"),
		Mgmtserviceaddress: "mgmt",
		Name:               "partition-1",
	}
	size1 = &models.V1SizeResponse{
		Constraints: []*models.V1SizeConstraint{
			{
				Max:  int64(2),
				Min:  int64(1),
				Type: new("storage"),
			},
			{
				Max:  int64(4),
				Min:  int64(3),
				Type: new("memory"),
			},
			{
				Max:  int64(6),
				Min:  int64(5),
				Type: new("cores"),
			},
		},
		Description: "size 1",
		ID:          new("1"),
		Name:        "size-1",
	}
	network1 = &models.V1NetworkResponse{
		Description:         "network 1",
		Destinationprefixes: []string{"dest"},
		ID:                  new("nw1"),
		Labels:              map[string]string{"a": "b"},
		Name:                "network-1",
		Nat:                 new(true),
		Parentnetworkid:     "",
		Partitionid:         "partition-1",
		Prefixes:            []string{"prefix"},
		Privatesuper:        new(true),
		Projectid:           "",
		Shared:              false,
		Underlay:            new(true),
		Usage: &models.V1NetworkUsage{
			AvailableIps:      new(int64(100)),
			AvailablePrefixes: new(int64(200)),
			UsedIps:           new(int64(300)),
			UsedPrefixes:      new(int64(400)),
		},
		Vrf:       50,
		Vrfshared: true,
	}
	firewall3 = &models.V1FirewallResponse{
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
			Created:          pointer.Pointer(strfmt.DateTime(testTime.Add(-20 * 24 * time.Hour))),
			Creator:          pointer.Pointer("creator"),
			Description:      "firewall allocation 3",
			Filesystemlayout: fsl1,
			Hostname:         pointer.Pointer("firewall-hostname-3"),
			Image:            image1,
			Name:             pointer.Pointer("firewall-3"),
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
			CrashLoop:            pointer.Pointer(true),
			FailedMachineReclaim: pointer.Pointer(true),
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
		ID: pointer.Pointer("3"),
		Ledstate: &models.V1ChassisIdentifyLEDState{
			Description: pointer.Pointer(""),
			Value:       pointer.Pointer(""),
		},
		Liveliness: pointer.Pointer("Unhealthy"),
		Name:       "firewall-3",
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
)

// we are sharing a client for the tests, so we need to make sure we do not run contradicting tests in parallel
// we can swap the client with this function
func swapMetalClient(mockFns *metalclient.MetalMockFns) {
	newClient, _ := metalclient.NewMetalMockClient(testingT, mockFns)

	if metalClient == nil {
		metalClient = newClient
		return
	}

	metalMockClient := metalClient.(*metalclient.MetalMockClient)
	*metalMockClient = *newClient // nolint for testing this is just fine
}
