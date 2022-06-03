package main

import (
	"context"
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/feloy/ododev/pkg"

	bindingApi "github.com/redhat-developer/service-binding-operator/apis/binding/v1alpha1"

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

	// register ServiceBinding resources
	mgr.GetClient().Scheme().AddKnownTypes(bindingApi.GroupVersion, &bindingApi.ServiceBinding{}, &bindingApi.ServiceBindingList{})
	metav1.AddToGroupVersion(mgr.GetClient().Scheme(), bindingApi.GroupVersion)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	devfilePath := filepath.Join(wd, "devfile.yaml")
	ctx := context.Background()
	err = pkg.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), devfilePath, namespace, componentName)
	if err != nil {
		panic(err)
	}

	devfileWatcher, err := pkg.NewDevfileWatcher(devfilePath)
	if err != nil {
		panic(err)
	}
	defer devfileWatcher.Close()

	for {
		select {
		case event, ok := <-devfileWatcher.Events:
			entryLog.Info("event")
			if !ok {
				return
			}
			entryLog.Info("modified file: " + event.Name)
			err = pkg.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), devfilePath, namespace, componentName)
			if err != nil {
				panic(err)
			}
		case err, ok := <-devfileWatcher.Errors:
			entryLog.Info("error")
			if !ok {
				return
			}
			entryLog.Info("error: ", err)
		}
		entryLog.Info("==========================================")
	}
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

	// Watch Deployments and enqueue owning ConfigMap key
	if err := c.Watch(&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.ConfigMap{}, IsController: true}); err != nil {
		return err
	}

	if err := mgr.Start( /*signals.SetupSignalHandler()*/ context.Background()); err != nil {
		return err
	}
	return nil
}
