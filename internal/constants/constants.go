package constants

import "errors"

const (
	OperatorName = "kubeturbo-operator"

	// recommended Kubernetes labels
	NameLabelKey      = "app.kubernetes.io/name"
	InstanceLabelKey  = "app.kubernetes.io/instance"
	PartOfLabelKey    = "app.kubernetes.io/part-of"
	ComponentLabelKey = "app.kubernetes.io/component"
	ManagedByLabelKey = "app.kubernetes.io/managed-by"
	CreatedByLabelKey = "app.kubernetes.io/created-by"

	KubeturboCRDName       = "kubeturbos.charts.helm.k8s.io"
	KubeturboContainerName = "kubeturbo"
	KubeturboComponentType = "kubeturbo"
	KubeturboAnnotation    = "charts.helm.k8s.io/kubeturbo"
	ControlGenAnnotation   = "controller-gen.kubebuilder.io/version"

	KubeturboFinalizer = "helm.k8s.io/finalizer"

	RequeueDelaySeconds = 1
	TimeoutInSeconds    = 5
)

var ErrRequeueOnDeletion = errors.New("resource deletion detected")
