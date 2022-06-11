package controller

import (
	"bytes"
	"context"
	"fmt"

	"github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/feloy/ododev/pkg/container"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func ExecDevfileCommand(
	ctx context.Context,
	client client.Client,
	mgr manager.Manager,
	pod *corev1.Pod,
	containerName string,
	targetPath string,
	cmd v1alpha2.Command,
) error {
	args := []string{"/bin/sh", "-c", fmt.Sprintf("(cd %s && %s) > /proc/1/fd/1 2> /proc/1/fd/2", targetPath, cmd.Exec.CommandLine)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	return container.Exec(ctx, client, mgr, pod, containerName, args, &stdout, &stderr, nil, false)
}
