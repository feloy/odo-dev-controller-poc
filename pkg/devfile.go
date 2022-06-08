package pkg

import (
	"context"
	"os"

	devfilev1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevfileSpecLabel = "devfile-spec"
)

func CreateConfigMapFromDevfile(ctx context.Context, client client.Client, filename string, namespace string, componentName string) (*corev1.ConfigMap, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"devfile": string(content),
		},
	}
	configMap.SetName("devfile-spec")
	configMap.SetNamespace(namespace)
	configMap.SetLabels(map[string]string{
		DevfileSpecLabel: componentName,
	})
	configMap.SetGeneration(1)

	err = client.Create(ctx, &configMap)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = client.Update(ctx, &configMap)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return &configMap, nil
}

func InfoFromDevfileConfigMap(ctx context.Context, client client.Client, cm corev1.ConfigMap) (*parser.DevfileObj, string, error) {
	content := cm.Data["devfile"]
	devfileObj, _, err := devfile.ParseDevfileAndValidate(parser.ParserArgs{
		Data: []byte(content),
	})
	if err != nil {
		return nil, "", err
	}
	return &devfileObj, cm.GetLabels()[DevfileSpecLabel], nil
}

// From odo/pkg/devfile
func GetKubernetesComponentsToPush(devfileObj parser.DevfileObj) ([]devfilev1.Component, error) {
	k8sComponents, err := devfileObj.Data.GetComponents(common.DevfileOptions{
		ComponentOptions: common.ComponentOptions{ComponentType: devfilev1.KubernetesComponentType},
	})
	if err != nil {
		return nil, err
	}

	componentsMap := map[string]devfilev1.Component{}
	for _, component := range k8sComponents {
		componentsMap[component.Name] = component
	}

	commands, err := devfileObj.Data.GetCommands(common.DevfileOptions{})
	if err != nil {
		return nil, err
	}

	for _, command := range commands {
		componentName := ""
		if command.Exec != nil {
			componentName = command.Exec.Component
		} else if command.Apply != nil {
			componentName = command.Apply.Component
		}
		if componentName == "" {
			continue
		}
		delete(componentsMap, componentName)
	}

	k8sComponents = make([]devfilev1.Component, len(componentsMap))
	i := 0
	for _, v := range componentsMap {
		k8sComponents[i] = v
		i++
	}

	return k8sComponents, err
}
