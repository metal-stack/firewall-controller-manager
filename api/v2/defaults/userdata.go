package defaults

import (
	"encoding/json"
	"fmt"

	"github.com/flatcar/container-linux-config-transpiler/config/types"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

const (
	FirewallControllerName = "firewall-controller"
	DroptailerClientName   = "droptailer"
)

func renderUserdata(kubeconfig, seedKubeconfig []byte) (string, error) {
	var (
		mode = 0600
		id   = 0

		cfg = types.Config{
			Systemd: types.Systemd{
				Units: []types.SystemdUnit{
					{
						Name:    fmt.Sprintf("%s.service", FirewallControllerName),
						Enable:  true,
						Enabled: pointer.Pointer(true),
					},
					{
						Name:    fmt.Sprintf("%s.service", DroptailerClientName),
						Enable:  true,
						Enabled: pointer.Pointer(true),
					},
				},
			},
			Storage: types.Storage{
				Files: []types.File{
					{
						Path:       fmt.Sprintf("/etc/%s/.kubeconfig", FirewallControllerName),
						Filesystem: "root",
						Mode:       &mode,
						User: &types.FileUser{
							Id: &id,
						},
						Group: &types.FileGroup{
							Id: &id,
						},
						Contents: types.FileContents{
							Inline: string(kubeconfig),
						},
					},
					{
						Path:       fmt.Sprintf("/etc/%s/.seed-kubeconfig", FirewallControllerName),
						Filesystem: "root",
						Mode:       &mode,
						User: &types.FileUser{
							Id: &id,
						},
						Group: &types.FileGroup{
							Id: &id,
						},
						Contents: types.FileContents{
							Inline: string(seedKubeconfig),
						},
					},
				},
			},
		}
	)

	outCfg, report := types.Convert(cfg, "", nil)
	if report.IsFatal() {
		return "", fmt.Errorf("could not transpile ignition config: %s", report.String())
	}

	userData, err := json.Marshal(outCfg)
	if err != nil {
		return "", err
	}

	return string(userData), nil
}
