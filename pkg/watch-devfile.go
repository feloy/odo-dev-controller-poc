package pkg

import (
	"github.com/fsnotify/fsnotify"
)

func NewDevfileWatcher() (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add("devfile.yaml")
	if err != nil {
		return nil, err
	}
	return watcher, nil
}
