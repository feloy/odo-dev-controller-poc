package filesystem

import (
	"github.com/fsnotify/fsnotify"
)

func NewDevfileWatcher(filename string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(filename)
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
