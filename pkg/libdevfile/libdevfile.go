package libdevfile

import (
	"fmt"
	"net"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"
)

// GetDefaultCommand returns the default command of the given kind in the devfile.
// If only one command of the kind exists, it is returned, even if it is not marked as default
func GetDefaultCommand(
	devfileObj parser.DevfileObj,
	kind v1alpha2.CommandGroupKind,
) (v1alpha2.Command, error) {
	groupCmds, err := devfileObj.Data.GetCommands(common.DevfileOptions{
		CommandOptions: common.CommandOptions{
			CommandGroupKind: kind,
		},
	})
	if err != nil {
		return v1alpha2.Command{}, err
	}
	if len(groupCmds) == 0 {
		return v1alpha2.Command{}, NewNoCommandFoundError(kind)
	}
	if len(groupCmds) > 1 {
		var found bool
		var foundGroupCmd v1alpha2.Command
		for _, groupCmd := range groupCmds {
			group := common.GetGroup(groupCmd)
			if group == nil {
				continue
			}
			if group.IsDefault != nil && *group.IsDefault {
				if found {
					return v1alpha2.Command{}, NewMoreThanOneDefaultCommandFoundError(kind)
				}
				found = true
				foundGroupCmd = groupCmd
			}
		}
		if !found {
			return v1alpha2.Command{}, NewNoDefaultCommandFoundError(kind)
		}
		return foundGroupCmd, nil
	}
	return groupCmds[0], nil
}

func GetPortPairs(devFileObj parser.DevfileObj) ([]string, error) {

	containers, err := devFileObj.Data.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{ComponentType: v1alpha2.ContainerComponentType},
	})
	if err != nil {
		return nil, err
	}

	ceMapping := make(map[string][]int)
	for _, container := range containers {
		if container.ComponentUnion.Container == nil {
			// this is not a container component; continue prevents panic when accessing Endpoints field
			continue
		}
		k := container.Name
		if _, ok := ceMapping[k]; !ok {
			ceMapping[k] = []int{}
		}

		endpoints := container.Container.Endpoints
		for _, e := range endpoints {
			if e.Exposure != v1alpha2.NoneEndpointExposure {
				ceMapping[k] = append(ceMapping[k], e.TargetPort)
			}
		}
	}

	portPairs := make(map[string][]string)
	port := 40000

	for name, ports := range ceMapping {
		for _, p := range ports {
			port++
			for {
				isPortFree := isPortFree(port)
				if isPortFree {
					pair := fmt.Sprintf("%d:%d", port, p)
					portPairs[name] = append(portPairs[name], pair)
					break
				}
				port++
			}
		}
	}

	var portPairsSlice []string
	for _, v1 := range portPairs {
		portPairsSlice = append(portPairsSlice, v1...)
	}

	return portPairsSlice, nil
}

func isPortFree(port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = listener.Addr().(*net.TCPAddr).Port
	err = listener.Close()
	return err == nil
}
