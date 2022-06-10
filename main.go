package main

import (
	"context"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/feloy/ododev/pkg"
	"github.com/feloy/ododev/pkg/controller"
	"github.com/feloy/ododev/pkg/devfile"

	bindingApi "github.com/redhat-developer/service-binding-operator/apis/binding/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func init() {
	log.SetLogger(zap.New())
}

func main() {
	var (
		namespace     = "project1"
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
		err := controller.StartManager(mgr, namespace, componentName)
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
	_, err = devfile.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), devfilePath, namespace, componentName)
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
			_, err = devfile.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), devfilePath, namespace, componentName)
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
