package controller

import (
	"context"
	"os"
	"strconv"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/feloy/ododev/pkg/container"
	"github.com/feloy/ododev/pkg/devfile"
	"github.com/feloy/ododev/pkg/libdevfile"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

type ReconcileConfigmap struct {
	Client  client.Client
	Manager manager.Manager
}

var _ reconcile.Reconciler = &ReconcileConfigmap{}

func (r *ReconcileConfigmap) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

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

	devfileObj, componentName, completeSyncModTime, err := devfile.InfoFromDevfileConfigMap(ctx, r.Client, cm)
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
		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
			Status:                devfile.StatusWaitDeployment,
			SyncedCompleteModTime: pointer.Int64(0),
		})
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// state: Deployment has a Running pod
	log.Info("running pod")

	// Check if all servicebindings' InjectionReady is true
	allInjected, err := checkServiceBindingsInjectionDone(ctx, r.Client, request.Namespace, componentName)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !allInjected {
		log.Info("missing bindings")
		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
			Status: devfile.StatusWaitBindings,
		})
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// state: All bindings are injected
	log.Info("all bindings injected")

	pod, err := getPod(ctx, r.Client, request.Namespace, componentName)
	if err != nil {
		return reconcile.Result{}, err
	}
	err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
		Status: devfile.StatusPodRunning,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// TODO sync files, exec commands, etc

	status, err := devfile.GetStatus(ctx, r.Client, request.Namespace, componentName)
	if err != nil {
		return reconcile.Result{}, err
	}
	log.Info("get status", "status", status.Status, "synced modtime", status.SyncedCompleteModTime)

	if completeSyncModTime != nil && (status.SyncedCompleteModTime == nil || *completeSyncModTime > *status.SyncedCompleteModTime) {
		log.Info("syncing file to pod", "pod", pod.GetName(), "modtime", completeSyncModTime, "status modtime", strconv.FormatInt(pointer.Int64Deref(status.SyncedCompleteModTime, 0), 10))

		tarReader, err := os.Open(".odo/complete.tar")
		if err != nil {
			return reconcile.Result{}, err
		}
		defer tarReader.Close()

		err = container.ExtractTarToContainer(ctx, r.Client, r.Manager, pod, "runtime", "/projects", tarReader) // TODO container name, target path
		if err != nil {
			return reconcile.Result{}, err
		}

		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
			Status:                devfile.StatusFilesSynced,
			SyncedCompleteModTime: completeSyncModTime,
		})
		if err != nil {
			return reconcile.Result{}, err
		}

		// build command
		buildCmd, err := libdevfile.GetDefaultCommand(*devfileObj, v1alpha2.BuildCommandGroupKind)
		if err != nil {
			return reconcile.Result{}, err
		}
		err = ExecDevfileCommand(ctx, r.Client, r.Manager, pod, "runtime", "/projects", buildCmd)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
			Status:                devfile.StatusBuildCommandExecuted,
			SyncedCompleteModTime: completeSyncModTime,
		})
		if err != nil {
			return reconcile.Result{}, err
		}

		// run command
		runCmd, err := libdevfile.GetDefaultCommand(*devfileObj, v1alpha2.RunCommandGroupKind)
		if err != nil {
			return reconcile.Result{}, err
		}

		go func() {
			_ = devfile.SetStatus(ctx, r.Client, request.Namespace, componentName, ownerRef, devfile.StatusContent{
				Status:                devfile.StatusRunCommandExecuted,
				SyncedCompleteModTime: completeSyncModTime,
			})
			err = ExecDevfileCommand(ctx, r.Client, r.Manager, pod, "runtime", "/projects", runCmd)
			if err != nil {
				log.Info("terminate run command with err", "err", err)
			} else {
				log.Info("terminate run command normally")
			}
		}()

	}

	return reconcile.Result{}, nil
}
