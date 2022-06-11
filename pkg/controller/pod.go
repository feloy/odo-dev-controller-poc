package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func getPod(ctx context.Context, client pkgclient.Client, namespace string, componentName string) (*corev1.Pod, error) {

	var list corev1.PodList
	err := client.List(ctx, &list,
		pkgclient.InNamespace(namespace),
		pkgclient.MatchingLabels{"component": componentName},
		// pkgclient.MatchingFields{"status.phase": "Running"}, // TODO get error "Index with name field:status.phase does not exist"
	)
	if err != nil {
		return nil, err
	}

	count := 0
	found := -1
	for i, pod := range list.Items {
		if pod.Status.Phase == "Running" {
			count++
			found = i
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("%d pods found", count)
	}
	return &list.Items[found], nil
}
