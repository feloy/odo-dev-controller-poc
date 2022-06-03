package pkg

import (
	"context"
	"os"

	"github.com/devfile/library/pkg/devfile"
	"github.com/devfile/library/pkg/devfile/parser"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevfileSpecLabel = "devfile-spec"
)

func CreateConfigMapFromDevfile(ctx context.Context, client client.Client, filename string, namespace string, componentName string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	configMap := corev1.ConfigMap{
		Data: map[string]string{
			"devfile": string(content),
		},
	}
	configMap.SetName("devfile-spec")
	configMap.SetNamespace(namespace)
	configMap.SetLabels(map[string]string{
		DevfileSpecLabel: componentName,
	})
	configMap.SetGeneration(1)

	err = client.Create(context.Background(), &configMap)
	if err != nil {
		return err
	}
	return nil
}

func InfoFromDevfileConfigMap(ctx context.Context, client client.Client, cm corev1.ConfigMap) (*parser.DevfileObj, string, error) {
	content := cm.Data["devfile"]
	devfileObj, _, err := devfile.ParseDevfileAndValidate(parser.ParserArgs{
		Data: []byte(content),
	})
	if err != nil {
		return nil, "", err
	}
	return &devfileObj, cm.GetLabels()[DevfileSpecLabel], nil
}
