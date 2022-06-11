package container

import (
	"bytes"
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func ExtractTarToContainer(ctx context.Context, client client.Client, mgr manager.Manager, pod *corev1.Pod, containerName string, targetPath string, stdin io.Reader) error {

	entryLog := log.Log.WithName("watch")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmdArr := []string{"mkdir", "-p", targetPath}
	err := Exec(ctx, client, mgr, pod, containerName, cmdArr, &stdout, &stderr, nil, false)
	if err != nil {
		entryLog.Info("error mkdir targetPath", "stdout", stdout.String(), "stderr", stderr.String())
		return err
	}

	// cmdArr will run inside container
	cmdArr = []string{"tar", "xf", "-", "-C", targetPath, "--no-same-owner"}
	err = Exec(ctx, client, mgr, pod, containerName, cmdArr, &stdout, &stderr, stdin, false)
	if err != nil {
		entryLog.Info("error mkdir targetPath", "stdout", stdout.String(), "stderr", stderr.String())
	}
	return err
}
