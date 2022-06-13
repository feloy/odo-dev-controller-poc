package container

import (
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func SetupPortForwarding(mgr manager.Manager, client client.Client, pod *corev1.Pod, portPairs []string, out io.Writer, errOut io.Writer) (chan struct{}, error) {
	transport, upgrader, err := spdy.RoundTripperFor(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	podGVK := corev1.SchemeGroupVersion.WithKind("Pod")

	rest, err := apiutil.RESTClientForGVK(podGVK, true, mgr.GetConfig(), serializer.NewCodecFactory(mgr.GetScheme()))
	if err != nil {
		return nil, err
	}

	req := rest.
		Post().
		Resource("pods").
		Namespace(pod.GetNamespace()).
		Name(pod.GetName()).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	stopChan := make(chan struct{}, 1)
	// passing nil for readyChan because it's eventually being closed if it's not nil
	// passing nil for out because we only care for error, not for output messages; we want to print our own messages
	fw, err := portforward.New(dialer, portPairs, stopChan, nil, out, errOut)
	if err != nil {
		return nil, err
	}

	// start port-forwarding
	go func() {
		fw.ForwardPorts()
	}()

	return stopChan, nil
}
