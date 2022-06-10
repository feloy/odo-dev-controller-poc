package controller

import (
	"context"
	"fmt"
	"strconv"

	"github.com/feloy/ododev/pkg/devfile"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

type ReconcileConfigmap struct {
	Client client.Client
}

var _ reconcile.Reconciler = &ReconcileConfigmap{}

func (r *ReconcileConfigmap) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	fmt.Printf("got request for %s\n", request.String())

	// Get the configmap containing the Devfile
	var cm corev1.ConfigMap
	err := r.Client.Get(ctx, request.NamespacedName, &cm)
	if errors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}

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

	devfileObj, componentName, err := devfile.InfoFromDevfileConfigMap(ctx, r.Client, cm)
	if err != nil {
		log.Error(err, "getting devfile from configmap")
		return reconcile.Result{}, err
	}

	// Apply the Kubernetes components
	k8sComponents, err := devfile.GetKubernetesComponentsToPush(*devfileObj)
	if err != nil {
		log.Error(err, "getting Kubernetes components to push")
		return reconcile.Result{}, err
	}
	if len(k8sComponents) == 0 {
		log.Info("no Kubernetes component to push")
	}
	for _, k8sc := range k8sComponents {
		log.Info("pushing component " + k8sc.Name)

	}
	err = pushKubernetesComponents(ctx, r.Client, k8sComponents, request.Namespace, ownerRef)
	if err != nil {
		log.Error(err, "pushing Kubernetes resources")
		return reconcile.Result{}, err
	}

	// Compute the expected deployment
	var newDep *appsv1.Deployment
	newDep, err = buildDeployment(*devfileObj, componentName, request.Namespace)
	if err != nil {
		log.Error(err, "building deployment resource")
		return reconcile.Result{}, err
	}
	newDep.SetOwnerReferences(append(newDep.GetOwnerReferences(), ownerRef))

	// Get the deployment for dev
	var dep appsv1.Deployment

	err = r.Client.Get(ctx, types.NamespacedName{
		Namespace: request.Namespace,
		Name:      getDeploymentName(componentName),
	}, &dep)

	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	// patch deployment and if updated, return

	err = r.Client.Patch(ctx, newDep, client.Apply, client.FieldOwner("ododev"), client.ForceOwnership)
	if err != nil {
		return reconcile.Result{}, err
	}

	log.Info("newDep generation: " + strconv.Itoa(int(newDep.Generation)))

	if dep.Generation < newDep.Generation {
		log.Info("Updated deployment",
			"previous generation", dep.Generation,
			"new generation", newDep.GenerateName,
		)
		return reconcile.Result{}, nil
	}

	// state: Deployment exists

	log.Info("deployment exists",
		"avail replicas", dep.Status.AvailableReplicas,
		"ready replicas", dep.Status.ReadyReplicas,
		"replicas", dep.Status.Replicas,
		"updated replicas", dep.Status.UpdatedReplicas,
	)

	if dep.Status.AvailableReplicas < 1 {
		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusWaitDeployment)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// state: Deployment has a Running pod

	// Check if all servicebindings' InjectionReady is true
	allInjected, err := checkServiceBindingsInjectionDone(ctx, r.Client, request.Namespace, componentName)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !allInjected {
		log.Info("missing bindings")
		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusWaitBindings)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// state: All bindings are injected
	log.Info("all bindings injected")

	err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusReady)
	if err != nil {
		return reconcile.Result{}, err
	}

	// TODO sync files, exec commands, etc
	return reconcile.Result{}, nil
}
