package devfile

import (
	"context"
	"os"
	"strconv"

	devfilev1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/generator"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	StatusWaitDeployment       Status = "WaitDeployment"
	StatusWaitBindings         Status = "WaitBindings"
	StatusPodRunning           Status = "PodRunning"
	StatusFilesSynced          Status = "FilesSynced"
	StatusBuildCommandExecuted Status = "BuildCommandExecuted"
	StatusRunCommandExecuted   Status = "RunCommandExecuted"
	StatusReady                Status = "Ready"
)

type ConfigMapContent struct {
	Devfile             string
	CompleteSyncModTime int64
}

type StatusContent struct {
	Status                Status
	SyncedCompleteModTime *int64
}

func CreateConfigMapFromDevfile(ctx context.Context, client client.Client, namespace string, componentName string, cmContent ConfigMapContent) (*corev1.ConfigMap, error) {
	content, err := os.ReadFile(cmContent.Devfile)
	if err != nil {
		return nil, err
	}
	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"devfile":             string(content),
			"completeSyncModTime": strconv.FormatInt(cmContent.CompleteSyncModTime, 10),
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

func InfoFromDevfileConfigMap(ctx context.Context, client client.Client, cm corev1.ConfigMap) (*parser.DevfileObj, string, *int64, error) {
	content := cm.Data["devfile"]
	devfileObj, _, err := devfile.ParseDevfileAndValidate(parser.ParserArgs{
		Data: []byte(content),
	})
	if err != nil {
		return nil, "", nil, err
	}
	var completeSyncModTime *int64
	if val, ok := cm.Data["completeSyncModTime"]; ok {
		var modTime int64
		modTime, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, "", nil, err
		}
		completeSyncModTime = &modTime
	}
	return &devfileObj, cm.GetLabels()[DevfileSpecLabel], completeSyncModTime, nil
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

func SetStatus(ctx context.Context, client client.Client, namespace string, componentName string, ownerRef metav1.OwnerReference, status StatusContent) error {

	oldStatus, _ := GetStatus(ctx, client, namespace, componentName)

	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"status": string(status.Status),
		},
	}
	if status.SyncedCompleteModTime != nil {
		configMap.Data["syncedCompleteModTime"] = strconv.FormatInt(*status.SyncedCompleteModTime, 10)
	} else if oldStatus.SyncedCompleteModTime != nil {
		configMap.Data["syncedCompleteModTime"] = strconv.FormatInt(*oldStatus.SyncedCompleteModTime, 10)
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

func GetStatus(ctx context.Context, client client.Client, namespace string, componentName string) (StatusContent, error) {
	cmKey := types.NamespacedName{
		Namespace: namespace,
		Name:      devfileStatusName,
	}
	var cm corev1.ConfigMap
	err := client.Get(ctx, cmKey, &cm)
	if err != nil {
		return StatusContent{}, err
	}

	var syncedCompleteModTime *int64
	if val, ok := cm.Data["syncedCompleteModTime"]; ok {
		var modTime int64
		modTime, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			return StatusContent{}, err
		}
		syncedCompleteModTime = &modTime
	}
	return StatusContent{
		Status:                Status(cm.Data["status"]),
		SyncedCompleteModTime: syncedCompleteModTime,
	}, nil
}

func WatchStatus(ctx context.Context, client client.Client, mgr manager.Manager, namespace string, componentName string, newStatus func(status string)) error {
	cmGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")

	rest, err := apiutil.RESTClientForGVK(cmGVK, true, mgr.GetConfig(), serializer.NewCodecFactory(mgr.GetScheme()))
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{
		Watch:         true,
		FieldSelector: "metadata.name=devfile-status",
	}

	watcher, err := rest.Get().
		Resource("configmaps").
		Namespace(namespace).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)

	if err != nil {
		return err
	}
	for event := range watcher.ResultChan() {
		switch obj := event.Object.(type) {
		case *corev1.ConfigMap:
			status := obj.Data["status"]
			newStatus(status)
		case *metav1.Status:
		}
	}
	return nil
}
