package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/feloy/ododev/pkg/devfile"
)

func StartManager(mgr manager.Manager, namespace string, componentName string) error {

	c, err := controller.New("devfile-controller", mgr, controller.Options{
		Reconciler: &ReconcileConfigmap{Client: mgr.GetClient()},
	})
	if err != nil {
		return err
	}

	configMapPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// The object doesn't contain label "devfile-spec=<component-name>", so the event will be
			// ignored.
			if cmp, ok := e.ObjectNew.GetLabels()[devfile.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}

			return e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			if cmp, ok := e.Object.GetLabels()[devfile.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if cmp, ok := e.Object.GetLabels()[devfile.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if cmp, ok := e.Object.GetLabels()[devfile.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
	}

	if err := c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, configMapPredicate); err != nil {
		return err
	}

	// Watch Deployments and enqueue owning ConfigMap key
	if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.ConfigMap{}, IsController: true}); err != nil {
		return err
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return err
	}
	return nil
}
