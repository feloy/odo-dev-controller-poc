package controller

import (
	"context"

	bindingApis "github.com/redhat-developer/service-binding-operator/apis"
	bindingApi "github.com/redhat-developer/service-binding-operator/apis/binding/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkServiceBindingsInjectionDone(ctx context.Context, cli client.Client, namespace string, componentName string) (bool, error) {

	var list bindingApi.ServiceBindingList
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	err := cli.List(ctx, &list, opts...)
	if err != nil {
		// If ServiceBinding kind is not registered => all bindings are done
		if runtime.IsNotRegisteredError(err) {
			return true, nil
		}
		return false, err
	}

	for _, binding := range list.Items {
		app := binding.Spec.Application
		if app.Group != appsv1.SchemeGroupVersion.Group || app.Version != appsv1.SchemeGroupVersion.Version || app.Kind != "Deployment" {
			continue
		}
		if injected := meta.IsStatusConditionTrue(binding.Status.Conditions, bindingApis.InjectionReady); !injected {
			return false, nil
		}
	}
	return true, nil
}
