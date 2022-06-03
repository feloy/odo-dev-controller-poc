package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/feloy/ododev/pkg"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	var (
		namespace     = "prj1"
		componentName = "my-go-app"
	)

	entryLog := log.Log.WithName("entrypoint")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		panic(err)
	}

	go func() {
		entryLog.Info("starting manager")
		err := startManager(mgr, namespace, componentName)
		if err != nil {
			panic(err)
		}
	}()

	ctx := context.Background()
	err = pkg.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), "devfile.yaml", namespace, componentName)
	if err != nil {
		panic(err)
	}

	select {}
}

func startManager(mgr manager.Manager, namespace string, componentName string) error {

	c, err := controller.New("devfile-controller", mgr, controller.Options{
		Reconciler: &pkg.ReconcileConfigmap{Client: mgr.GetClient()},
	})
	if err != nil {
		return err
	}

	configMapPredicate := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// The object doesn't contain label "devfile-spec=<component-name>", so the event will be
			// ignored.
			if cmp, ok := e.ObjectNew.GetLabels()[pkg.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}

			return e.ObjectOld != e.ObjectNew
		},
		CreateFunc: func(e event.CreateEvent) bool {
			if cmp, ok := e.Object.GetLabels()[pkg.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if cmp, ok := e.Object.GetLabels()[pkg.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			if cmp, ok := e.Object.GetLabels()[pkg.DevfileSpecLabel]; !ok || cmp != componentName {
				return false
			}
			return true
		},
	}

	if err := c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, configMapPredicate); err != nil {
		return err
	}

	if err := mgr.Start( /*signals.SetupSignalHandler()*/ context.Background()); err != nil {
		return err
	}
	return nil
}