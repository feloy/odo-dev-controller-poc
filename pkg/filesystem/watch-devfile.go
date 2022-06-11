package filesystem

import "github.com/rjeczalik/notify"

func NewDevfileWatcher(filename string) (chan notify.EventInfo, error) {
	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(filename, c, notify.InCloseWrite, notify.InDelete); err != nil {
		return nil, err
	}

	return c, nil
}
