package controller

import (
	"github.com/devfile/library/pkg/devfile/generator"
	"github.com/devfile/library/pkg/devfile/parser"
	"github.com/devfile/library/pkg/devfile/parser/data/v2/common"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/pointer"
)

func buildDeployment(devfileObj parser.DevfileObj, componentName string, namespace string) (*appsv1.Deployment, error) {
	containers, err := generator.GetContainers(devfileObj, common.DevfileOptions{})
	if err != nil {
		return nil, err
	}

	initContainers, err := generator.GetInitContainers(devfileObj)
	if err != nil {
		return nil, err
	}

	selectorLabels := map[string]string{
		"component": componentName,
	}

	deploymentObjectMeta := generator.GetObjectMeta(componentName, namespace, selectorLabels, nil)

	apiVersion, kind := appsv1.SchemeGroupVersion.WithKind("Deployment").ToAPIVersionAndKind()
	return generator.GetDeployment(devfileObj, generator.DeploymentParams{
		TypeMeta:          generator.GetTypeMeta(kind, apiVersion),
		ObjectMeta:        deploymentObjectMeta,
		InitContainers:    initContainers,
		Containers:        containers,
		PodSelectorLabels: selectorLabels,
		Replicas:          pointer.Int32Ptr(1),
	})
}
