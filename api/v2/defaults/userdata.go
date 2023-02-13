package defaults

import (
	"encoding/json"
	"fmt"

	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"

	"github.com/flatcar/container-linux-config-transpiler/config/types"
)

const (
	FirewallControllerName = "firewall-controller"
	DroptailerClientName   = "droptailer"
)

func createUserdata(c *helper.SeedAccessConfig) (string, error) {
	kubeconfig, err := helper.SeedAccessKubeconfig(c)
	if err != nil {
		return "", err
	}

	return renderUserdata(kubeconfig)
}

func renderUserdata(kubeconfig []byte) (string, error) {
	cfg := types.Config{}
	cfg.Systemd = types.Systemd{}

	enabled := true
	fcUnit := types.SystemdUnit{
		Name:    fmt.Sprintf("%s.service", FirewallControllerName),
		Enable:  enabled,
		Enabled: &enabled,
	}
	dcUnit := types.SystemdUnit{
		Name:    fmt.Sprintf("%s.service", DroptailerClientName),
		Enable:  enabled,
		Enabled: &enabled,
	}

	cfg.Systemd.Units = append(cfg.Systemd.Units, fcUnit, dcUnit)

	cfg.Storage = types.Storage{}

	mode := 0600
	id := 0
	ignitionFile := types.File{
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
	}
	cfg.Storage.Files = append(cfg.Storage.Files, ignitionFile)

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
