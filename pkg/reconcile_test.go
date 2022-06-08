package pkg

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("Static controller", func() {

	const (
		timeout  = time.Second * 600
		interval = time.Second * 1
	)

	var (
		ctx = context.Background()

		deploymentKey = types.NamespacedName{
			Name:      componentName,
			Namespace: namespace,
		}
	)

	When("a Devfile configmap is created", func() {

		var (
			created                corev1.ConfigMap
			expectedOwnerReference metav1.OwnerReference
		)

		BeforeEach(func() {
			cm, err := CreateConfigMapFromDevfile(ctx, k8sClient, "tests/devfile.yaml", namespace, componentName)
			Expect(err).Should(Succeed())

			expectedOwnerReference = metav1.OwnerReference{
				Kind:               "ConfigMap",
				APIVersion:         "v1",
				Name:               "devfile-spec",
				UID:                cm.UID,
				Controller:         pointer.Bool(true),
				BlockOwnerDeletion: nil,
			}
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, &created)
		})

		Specify("a deployment is created", func() {

			By("creating a deployment owned by the configmap", func() {
				var deployment appsv1.Deployment
				Eventually(func() error {
					return k8sClient.Get(ctx, deploymentKey, &deployment)
				}, timeout, interval).Should(BeNil())
				Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
			})
		})

		When("the deployment has one available replica", func() {

			BeforeEach(func() {
				var deployment appsv1.Deployment
				Expect(k8sClient.Get(ctx, deploymentKey, &deployment)).Should(Succeed())
				deployment.Status.Replicas = 1
				deployment.Status.ReadyReplicas = 1
				deployment.Status.AvailableReplicas = 1
				Expect(k8sClient.Status().Update(ctx, &deployment)).Should(Succeed())
			})

			Specify("", func() {
				time.Sleep(10 * time.Second)
			})
		})
	})
})
