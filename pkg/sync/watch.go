package sync

import (
	"path/filepath"
	"time"

	"github.com/feloy/ododev/pkg/filesystem"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/rjeczalik/notify"
	gitignore "github.com/sabhiram/go-gitignore"
)

func Watch(
	devfilePath string,
	wd string,
	ignoreMatcher *gitignore.GitIgnore,
	statusWatcher <-chan watch.Event,
	updatedStatus func(status string),
	modifiedDevfile func() error,
	modifiedSources func(deleted []string, modified []string) error,
) error {

	devfileWatcher, err := filesystem.NewDevfileWatcher(devfilePath)
	if err != nil {
		return err
	}
	defer notify.Stop(devfileWatcher)

	sourcesWatcher, err := filesystem.NewSourcesWatcher(wd)
	if err != nil {
		return err
	}
	defer notify.Stop(sourcesWatcher)

	deleted := map[string]struct{}{}
	modified := map[string]struct{}{}
	timer := time.NewTimer(time.Millisecond)
	<-timer.C

	for {
		select {

		case <-devfileWatcher:
			err = modifiedDevfile()
			if err != nil {
				return err
			}

		case notif := <-sourcesWatcher:
			event := notif.Event()
			path := notif.Path()
			rel, err := filepath.Rel(wd, path)
			if err != nil {
				return err
			}
			if matched := ignoreMatcher.MatchesPath(rel); matched {
				continue
			}
			switch event {
			case notify.InCloseWrite:
				modified[rel] = struct{}{}
			case notify.InDelete:
				deleted[rel] = struct{}{}
			}
			timer.Reset(100 * time.Millisecond)

		case <-timer.C:
			modifiedSources(mapKeysToSlice(deleted), mapKeysToSlice(modified))
			deleted = map[string]struct{}{}
			modified = map[string]struct{}{}

		case event := <-statusWatcher:
			switch obj := event.Object.(type) {
			case *corev1.ConfigMap:
				status := obj.Data["status"]
				updatedStatus(status)
			}

		}

	}
}
func mapKeysToSlice(m map[string]struct{}) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
