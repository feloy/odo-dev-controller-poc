package devfile

import (
	"context"
	"os"

	devfilev1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/generator"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// devfileSpecName is the name of the configmap containing the spec (the devfile)
	devfileSpecName = "devfile-spec"

	// devfileStatusName is the name of the configmap containing the status
	devfileStatusName = "devfile-status"

	// DevfileSpecLabel is the label set to configmap
	DevfileSpecLabel = "devfile-spec"

	// DevfileStatusLabel is the label set to configmap
	DevfileStatusLabel = "devfile-status"
)

type Status string

const (
	StatusWaitDeployment Status = "WaitDeployment"
	StatusWaitBindings   Status = "WaitBindings"
	StatusReady          Status = "Ready"
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
	configMap.SetName(devfileSpecName)
	configMap.SetNamespace(namespace)
	configMap.SetLabels(map[string]string{
		DevfileSpecLabel: componentName,
	})
	configMap.APIVersion, configMap.Kind = corev1.SchemeGroupVersion.WithKind("ConfigMap").ToAPIVersionAndKind()

	err = client.Patch(ctx, &configMap, pkgclient.Apply, pkgclient.FieldOwner("ododev"), pkgclient.ForceOwnership)
	if err != nil {
		return nil, err
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

func SetStatus(ctx context.Context, client client.Client, namespace string, componentName string, ownerRef metav1.OwnerReference, status Status) error {
	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"status": string(status),
		},
	}
	apiVersion, kind := corev1.SchemeGroupVersion.WithKind("ConfigMap").ToAPIVersionAndKind()
	configMap.TypeMeta = generator.GetTypeMeta(kind, apiVersion)
	configMap.SetName(devfileStatusName)
	configMap.SetNamespace(namespace)
	configMap.SetLabels(map[string]string{
		DevfileStatusLabel: componentName,
	})
	configMap.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	err := client.Patch(ctx, &configMap, pkgclient.Apply, pkgclient.FieldOwner("ododev"))
	return err
}

func GetStatus(ctx context.Context, client client.Client, namespace string, componentName string) (Status, error) {
	cmKey := types.NamespacedName{
		Namespace: namespace,
		Name:      devfileStatusName,
	}
	var cm corev1.ConfigMap
	err := client.Get(ctx, cmKey, &cm)
	if err != nil {
		return "", err
	}
	return Status(cm.Data["status"]), nil
}
