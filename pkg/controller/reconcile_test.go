package controller

import (
	"context"
	"time"

	"github.com/feloy/ododev/pkg/devfile"
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
		podTimeout = time.Second * 120
		timeout    = time.Second * 10
		interval   = time.Second * 1
	)

	var (
		ctx = context.Background()

		deploymentKey = types.NamespacedName{
			Name:      componentName,
			Namespace: namespace,
		}

		devfileSpecKey = types.NamespacedName{
			Name:      "devfile-spec",
			Namespace: namespace,
		}
	)

	for _, test := range []struct {
		name            string
		devfile         string
		modifiedDevfile string
	}{
		{
			name:            "a Devfile configmap is created",
			devfile:         "tests/devfile.yaml",
			modifiedDevfile: "tests/devfile-edit1.yaml",
		},
		{
			name:            "a Devfile configmap is created, with a service binding",
			devfile:         "tests/devfile-binding.yaml",
			modifiedDevfile: "tests/devfile-binding-edit1.yaml",
		},
	} {
		When(test.name, func() {

			var (
				created                *corev1.ConfigMap
				expectedOwnerReference metav1.OwnerReference
			)

			BeforeEach(func() {
				// wait previous configmap to be deleted
				Eventually(func() error {
					var cm corev1.ConfigMap
					return k8sClient.Get(ctx, devfileSpecKey, &cm)
				}, timeout, interval).ShouldNot(BeNil())

				var err error
				created, err = devfile.CreateConfigMapFromDevfile(ctx, k8sClient, test.devfile, namespace, componentName)
				Expect(err).Should(Succeed())

				expectedOwnerReference = metav1.OwnerReference{
					Kind:               "ConfigMap",
					APIVersion:         "v1",
					Name:               "devfile-spec",
					UID:                created.UID,
					Controller:         pointer.Bool(true),
					BlockOwnerDeletion: nil,
				}
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(ctx, created)).Should(Succeed())
			})

			Specify("a deployment is created", func() {

				By("creating a deployment owned by the configmap", func() {
					var deployment appsv1.Deployment
					Eventually(func() error {
						return k8sClient.Get(ctx, deploymentKey, &deployment)
					}, timeout, interval).Should(BeNil())
					Expect(deployment.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
				})

				By("having a memory limit set to 512Mi", func() {
					var deployment appsv1.Deployment
					Eventually(func() error {
						return k8sClient.Get(ctx, deploymentKey, &deployment)
					}, timeout, interval).Should(BeNil())
					Expect(deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()).Should(Equal("512Mi"))
				})

				By("having a status of WaitDeployment", func() {
					Eventually(func() devfile.Status {
						status, _ := devfile.GetStatus(ctx, k8sClient, namespace, componentName)
						return status
					}, timeout, interval).Should(Equal(devfile.StatusWaitDeployment))
				})
			})

			When("the deployment has one available replica (by the deployment controller)", func() {

				//			BeforeEach(func() {
				//				var deployment appsv1.Deployment
				//				Expect(k8sClient.Get(ctx, deploymentKey, &deployment)).Should(Succeed())
				//				deployment.Status.Replicas = 1
				//				deployment.Status.ReadyReplicas = 1
				//				deployment.Status.AvailableReplicas = 1
				//				Expect(k8sClient.Status().Update(ctx, &deployment)).Should(Succeed())
				//			})

				Specify("the status of the devfile is Ready", func() {
					Eventually(func() devfile.Status {
						status, _ := devfile.GetStatus(ctx, k8sClient, namespace, componentName)
						return status
					}, podTimeout, interval).Should(Equal(devfile.StatusReady))
				})

				When("the Devfile is modified", func() {
					BeforeEach(func() {
						_, err := devfile.CreateConfigMapFromDevfile(ctx, k8sClient, test.modifiedDevfile, namespace, componentName)
						Expect(err).Should(Succeed())
					})

					Specify("the Deployment should be modified", func() {
						By("having a memory limit set to 1Gi", func() {
							var deployment appsv1.Deployment
							Eventually(func() string {
								err := k8sClient.Get(ctx, deploymentKey, &deployment)
								if err != nil {
									return ""
								}
								return deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String()
							}, timeout, interval).Should(Equal("1Gi"))
						})
					})

					When("the number of available replicas is set to zero (by the deployment controller)", func() {
						//BeforeEach(func() {
						//	Eventually(func() error {
						//		var deployment appsv1.Deployment
						//		Expect(k8sClient.Get(ctx, deploymentKey, &deployment)).Should(Succeed())
						//		deployment.Status.Replicas = 0
						//		deployment.Status.ReadyReplicas = 0
						//		deployment.Status.AvailableReplicas = 0
						//		return k8sClient.Status().Update(ctx, &deployment)
						//	}).Should(Succeed())
						//})

						Specify("the status should be WaitDeployment", func() {
							Eventually(func() devfile.Status {
								status, _ := devfile.GetStatus(ctx, k8sClient, namespace, componentName)
								return status
							}, timeout, interval).Should(Equal(devfile.StatusWaitDeployment))
						})

						When("the deployment has one available replica (by the deployment controller)", func() {
							//BeforeEach(func() {
							//	var deployment appsv1.Deployment
							//	Expect(k8sClient.Get(ctx, deploymentKey, &deployment)).Should(Succeed())
							//	deployment.Status.Replicas = 1
							//	deployment.Status.ReadyReplicas = 1
							//	deployment.Status.AvailableReplicas = 1
							//	Expect(k8sClient.Status().Update(ctx, &deployment)).Should(Succeed())
							//})

							Specify("the status of the devfile is Ready", func() {
								Eventually(func() devfile.Status {
									status, _ := devfile.GetStatus(ctx, k8sClient, namespace, componentName)
									return status
								}, podTimeout, interval).Should(Equal(devfile.StatusReady))
							})
						})
					})

				})
			})
		})
	}

})
