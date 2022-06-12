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
	args := []string{"/bin/sh", "-c", fmt.Sprintf("echo $$ > /tmp/odo_command.pid; (cd %s && %s) > /proc/1/fd/1 2> /proc/1/fd/2", targetPath, cmd.Exec.CommandLine)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := container.Exec(ctx, client, mgr, pod, containerName, args, &stdout, &stderr, nil, false)
	fmt.Println(stdout.String())
	fmt.Println(stderr.String())
	return err
}

func StopDevfileCommand(
	ctx context.Context,
	client client.Client,
	mgr manager.Manager,
	pod *corev1.Pod,
	containerName string,
) error {
	args := []string{"/bin/sh", "-c", `
ls /tmp/odo_command.pid >/dev/null 2> /dev/null && 
(
	PID=$(cat /tmp/odo_command.pid) && 
	while 
		kill -9 $(cat /proc/$PID/task/$PID/children 2> /dev/null) 2> /dev/null
	do
		sleep 0.1
	done && 
	rm -f /tmp/odo_command.pid
) || true`}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := container.Exec(ctx, client, mgr, pod, containerName, args, &stdout, &stderr, nil, false)
	fmt.Println(stdout.String())
	fmt.Println(stderr.String())
	return err
}
