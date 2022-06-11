package sync

import (
	"path/filepath"
	"time"

	"github.com/feloy/ododev/pkg/filesystem"

	"github.com/rjeczalik/notify"
	gitignore "github.com/sabhiram/go-gitignore"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Watch(
	devfilePath string,
	wd string,
	ignoreMatcher *gitignore.GitIgnore,
	modifiedDevfile func() error,
	modifiedSources func(deleted []string, modified []string) error,
) error {

	entryLog := log.Log.WithName("watch")

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

		case notif := <-devfileWatcher:
			entryLog.Info("devfile event")
			path := notif.Path()
			rel, err := filepath.Rel(wd, path)
			if err != nil {
				return err
			}
			entryLog.Info("modified file: " + rel)
			err = modifiedDevfile()
			if err != nil {
				return err
			}

		case notif := <-sourcesWatcher:
			entryLog.Info("sources event")
			event := notif.Event()
			path := notif.Path()
			rel, err := filepath.Rel(wd, path)
			if err != nil {
				return err
			}
			switch event {
			case notify.InCloseWrite:
				entryLog.Info("Editing of file is done", "file", rel)
			case notify.InDelete:
				entryLog.Info("File was deleted.", "file", rel)
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
			entryLog.Info("modified file: " + rel)
			timer.Reset(100 * time.Millisecond)

		case <-timer.C:
			modifiedSources(mapKeysToSlice(deleted), mapKeysToSlice(modified))
			deleted = map[string]struct{}{}
			modified = map[string]struct{}{}
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
