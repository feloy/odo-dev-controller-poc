package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/feloy/ododev/pkg/controller"
	"github.com/feloy/ododev/pkg/devfile"
	"github.com/feloy/ododev/pkg/filesystem"
	"github.com/feloy/ododev/pkg/sync"

	bindingApi "github.com/redhat-developer/service-binding-operator/apis/binding/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func main() {
	var (
		namespace       = "project1"
		componentName   = "my-go-app"
		dotOdoDirectory = ".odo"
		completeTarFile = filepath.Join(dotOdoDirectory, "complete.tar")
	)

	// Check .odo exists
	err := os.Mkdir(dotOdoDirectory, 0755)
	if err != nil {
		if !os.IsExist(err) {
			panic(err)
		}
	}

	f, err := os.Create(".odo/controller.log")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	log.SetLogger(zap.New(zap.WriteTo(f)))

	entryLog := log.Log.WithName("entrypoint")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Namespace: namespace,
	})
	if err != nil {
		panic(err)
	}

	ctx := signals.SetupSignalHandler()

	go func() {
		entryLog.Info("starting manager")
		err := controller.StartManager(ctx, mgr, namespace, componentName)
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

	ignoreMatcher, err := filesystem.GetIgnoreMatcher(wd)

	modTime, err := filesystem.Archive(wd, completeTarFile, ignoreMatcher)
	if err != nil {
		panic(err)
	}

	devfileConfigMap, err := devfile.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), namespace, componentName, devfile.ConfigMapContent{
		Devfile:             devfilePath,
		CompleteSyncModTime: modTime,
	})
	if err != nil {
		panic(err)
	}

	statusWatcher, err := devfile.WatchStatus(ctx, mgr.GetClient(), mgr, namespace, componentName)
	if err != nil {
		panic(err)
	}

	sync.Watch(ctx, devfilePath, wd, ignoreMatcher, statusWatcher,
		func(status string) {
			fmt.Printf("new status: %s\n", status)
		},
		func() error {
			_, err = devfile.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), namespace, componentName, devfile.ConfigMapContent{
				Devfile:             devfilePath,
				CompleteSyncModTime: modTime,
			})
			return err
		}, func(deleted []string, modified []string) error {
			if len(modified) > 0 {
				fmt.Printf("Files modified: %s\n", strings.Join(modified, ", "))
			}
			if len(deleted) > 0 {
				fmt.Printf("Files deleted: %s\n", strings.Join(deleted, ", "))
			}
			// TODO create complete + diff tar, update configmap
			modTime, err = filesystem.Archive(wd, completeTarFile, ignoreMatcher)
			if err != nil {
				panic(err)
			}
			_, err = devfile.CreateConfigMapFromDevfile(ctx, mgr.GetClient(), namespace, componentName, devfile.ConfigMapContent{
				Devfile:             devfilePath,
				CompleteSyncModTime: modTime,
			})
			return err
		})

	fmt.Println("Cleanup resources, please wait or press Ctrl-c again to not wait resource cleanup is done")
	// use a new context as the previous has been canceled
	err = devfile.DeleteConfigMapAndWait(context.Background(), mgr, mgr.GetClient(), devfileConfigMap)
	if err != nil {
		panic(err)
	}
}
