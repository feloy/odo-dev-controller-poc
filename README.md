# odo dev implementation using the controller-runtime library

odo dev actually uses multiple loops to synchronize elements:
- a first loop is used to deploy Kubernetes components to the cluster and, when a Pod is stable, files are synchronized to the pod's container and the program is executed into the container
- when the program is running, a second loop checks for files changes. When files are changed, the first loop is called again to reconcile the deployed resources if necessary and to re-synchronize the files and restart the program.

Because of these two separated loops, some events cannot be handled correctly:
- files changes when the resources are being deployed,
- resources reconciliation when no files are changed.

The use of a central and unique loop for all these events can help fix these problems.

## controller-runtime

The `controller-runtime` library helps implement Kubernetes controllers/operators. It is used by Kubebuiler and the Operator SDK.

The mode of operation of kubebuilder, and Kubernetes controllers/operators in general, is:

- the controller/operator watches for a specific primary resource kind, referred as the Specification. It is generally a CRD for operators managing a custom resource, or a Kubernetes native resource for native controllers (for example the Deployment resource kind for the Deployment controller)
- the controller/operator optionally watches for secondary resources, for which the operator has special interest (generally because it creates such resources).

The controller-runtime defines a `Reconcile` function, which is the central and unique function where all the synchronization happens, and which is called every time an object watched by the operator is added/modified/deleted.

The role of the Reconcile loop is to read the Specification included in the primary resource (CRD or other resource), and to execute the necessary steps to apply
these specifications to the cluster.

The Status part of the CRD/other resource is useful for the controller/operator to store the state of its work: 
- as several steps are generally necessary to access the specs, the operator can store at which intermediary step the specs is being reconciled
- the user, who has written the spec and is waiting for it to be reconciled on the cluster, can know the advancement and the status of the deployment.

## odo dev implementation

Operators managing a specific resource generally define a CustomResourceDefinition (CRD), defining the Specs. Declaring this CRD to the cluster needs specific rights, that odo users generally don't have. This implemntation uses the native ConfigMap resources to store the Specs and the Status.

The Specs are stored in a ConfigMap and are composed of:
- the devfile content, to help build the Kubernetes resources and forward ports
- an indication of the files to synchronize to the application's container

The Status is stored in a separate ConfigMap and is composed of:
- the state of the deployment of Kubernetes resources (aAitDeployment, WaitBindings, PodRunning, FilesSynced, BuildCommandExecuted, RunCommandRunning)
- the forwarded ports
- the state of the file synchronization

The `odo dev` is split in two co-routines:
- The "client" co-routine is watching for changes of the Devfile and sources files, and updates the Specs as soon as changes happen in the Devfile or the source code.It also watches to Status ConfigMap to inform the user with the status of the deployment, the forwarded ports, etc.
- the "controller" co-routine is watching for ConfigMap containing the Specs, and rollouts the steps to deploy the application to the cluster respecting the Devfile and with the up to date sources.
