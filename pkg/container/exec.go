package container

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ExecCMDInContainer execute command in the container of a pod, pass an empty string for containerName to execute in the first container of the pod
func Exec(ctx context.Context, client client.Client, mgr manager.Manager, pod *corev1.Pod, containerName string, cmd []string, stdout io.Writer, stderr io.Writer, stdin io.Reader, tty bool) error {

	podGVK := corev1.SchemeGroupVersion.WithKind("Pod")

	rest, err := apiutil.RESTClientForGVK(podGVK, true, mgr.GetConfig(), serializer.NewCodecFactory(mgr.GetScheme()))
	if err != nil {
		return err
	}

	podExecOptions := corev1.PodExecOptions{
		Command: cmd,
		Stdin:   stdin != nil,
		Stdout:  stdout != nil,
		Stderr:  stderr != nil,
		TTY:     tty,
	}

	// If a container name was passed in, set it in the exec options, otherwise leave it blank
	if containerName != "" {
		podExecOptions.Container = containerName
	}

	req := rest.
		Post().
		Namespace(pod.GetNamespace()).
		Resource("pods").
		Name(pod.GetName()).
		SubResource("exec").
		VersionedParams(&podExecOptions, scheme.ParameterCodec)

	config := mgr.GetConfig()

	// Connect to url (constructed from req) using SPDY (HTTP/2) protocol which allows bidirectional streams.
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("unable execute command via SPDY: %w", err)
	}
	// initialize the transport of the standard shell streams
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
	if err != nil {
		return fmt.Errorf("error while streaming command: %w", err)
	}

	return nil
}
