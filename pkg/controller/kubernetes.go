package controller

import (
	"context"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func pushKubernetesComponents(ctx context.Context, client client.Client, components []v1alpha2.Component, namespace string, ownerRef metav1.OwnerReference) error {

	// create an object on the kubernetes cluster for all the Kubernetes Inlined components
	for _, c := range components {
		yml := c.Kubernetes.Inlined // TODO call GetK8sManifestWithVariablesSubstituted
		var u unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yml), &u.Object)
		if err != nil {
			return err
		}
		u.SetNamespace(namespace)

		var prev unstructured.Unstructured
		prev.SetKind(u.GetKind())
		prev.SetAPIVersion(u.GetAPIVersion())
		err = client.Get(ctx, types.NamespacedName{
			Name:      u.GetName(),
			Namespace: namespace,
		}, &prev)
		if err != nil {
			if errors.IsNotFound(err) {
				u.SetOwnerReferences(append(u.GetOwnerReferences(), ownerRef))
				err = client.Create(ctx, &u)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// TODO patch it

	}

	// TODO delete components removed from devfile

	return nil
}
