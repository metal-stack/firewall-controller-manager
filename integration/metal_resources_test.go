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
	firewall2 = func(event, message string) *models.V1FirewallResponse {
		return &models.V1FirewallResponse{
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
				Created:          pointer.Pointer(strfmt.DateTime(testTime)),
				Creator:          pointer.Pointer("creator"),
				Description:      "firewall allocation 2",
				Filesystemlayout: fsl1,
				Hostname:         pointer.Pointer("firewall-hostname-2"),
				Image:            image1,
				Name:             pointer.Pointer("firewall-2"),
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
			Description: "firewall 2",
			Events: &models.V1MachineRecentProvisioningEvents{
				CrashLoop:            pointer.Pointer(false),
				FailedMachineReclaim: pointer.Pointer(false),
				LastErrorEvent:       &models.V1MachineProvisioningEvent{},
				LastEventTime:        strfmt.DateTime(testTime.Add(-1 * time.Minute)),
				Log: []*models.V1MachineProvisioningEvent{
					{
						Event:   pointer.Pointer(event),
						Message: message,
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
			ID: pointer.Pointer("2"),
			Ledstate: &models.V1ChassisIdentifyLEDState{
				Description: pointer.Pointer(""),
				Value:       pointer.Pointer(""),
			},
			Liveliness: pointer.Pointer("Alive"),
			Name:       "firewall-2",
			Partition:  partition1,
			Rackid:     "rack-1",
			Size:       size1,
			State: &models.V1MachineState{
				Description:        pointer.Pointer("state"),
				Issuer:             "issuer",
				MetalHammerVersion: pointer.Pointer("version"),
				Value:              pointer.Pointer(""),
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

// we are sharing a client for the tests, so we need to make sure we do not run contradicting tests in parallel
// we can swap the client with this function
func swapMetalClient(mockFns *metalclient.MetalMockFns) {
	newClient, _ := metalclient.NewMetalMockClient(mockFns)

	if metalClient == nil {
		metalClient = newClient
		return
	}

	metalMockClient := metalClient.(*metalclient.MetalMockClient)
	*metalMockClient = *newClient // nolint

	return
}
