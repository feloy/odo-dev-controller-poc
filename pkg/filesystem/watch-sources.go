package filesystem

import (
	"github.com/rjeczalik/notify"
)

func NewSourcesWatcher(directory string) (chan notify.EventInfo, error) {
	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(directory+"/...", c, notify.InCloseWrite, notify.InDelete); err != nil {
		return nil, err
	}

	return c, nil
}
