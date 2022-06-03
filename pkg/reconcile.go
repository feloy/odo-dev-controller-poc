package pkg

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type ReconcileConfigmap struct {
	// client can be used to retrieve objects from the APIServer.
	Client client.Client
}

var _ reconcile.Reconciler = &ReconcileConfigmap{}

func (r *ReconcileConfigmap) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	fmt.Printf("got request for %s\n", request.String())

	var cm corev1.ConfigMap
	err := r.Client.Get(ctx, request.NamespacedName, &cm)
	if err != nil {
		return reconcile.Result{}, err
	}
	ownerRef := metav1.OwnerReference{
		APIVersion: cm.APIVersion,
		Kind:       cm.Kind,
		Name:       cm.GetName(),
		UID:        cm.GetUID(),
		Controller: pointer.Bool(true),
	}

	devfileObj, componentName, err := InfoFromDevfileConfigMap(ctx, r.Client, cm)
	if err != nil {
		log.Error(err, "getting devfile from configmap")
		return reconcile.Result{}, err
	}

	dep, err := buildDeployment(*devfileObj, componentName, request.Namespace)
	if err != nil {
		log.Error(err, "building deployment resource")
		return reconcile.Result{}, err
	}

	dep.SetOwnerReferences(append(dep.GetOwnerReferences(), ownerRef))

	err = r.Client.Create(ctx, dep)
	if err != nil {
		yml, _ := yaml.Marshal(dep)
		fmt.Printf("%s\n", yml)
		log.Error(err, "creating deployment")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
