package kubeturbo

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeturbosv1 "github.ibm.com/turbonomic/kubeturbo-deploy/api/v1"
	consts "github.ibm.com/turbonomic/kubeturbo-deploy/internal/constants"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/request"
)

type KubeturboRequest struct {
	request.BaseRequest[*kubeturbosv1.Kubeturbo]
}

func NewKubeturboRequest(
	client client.Client,
	ctx context.Context,
	scheme *runtime.Scheme,
	kt *kubeturbosv1.Kubeturbo,
) *KubeturboRequest {

	return &KubeturboRequest{
		BaseRequest: request.BaseRequest[*kubeturbosv1.Kubeturbo]{
			Cr:      kt,
			Client:  client,
			Context: ctx,
			Scheme:  scheme,
		},
	}
}

func (kr *KubeturboRequest) ReleaseLabels() map[string]string {
	return map[string]string{
		consts.InstanceLabelKey:  kr.Instance(),
		consts.PartOfLabelKey:    kr.Name(),
		consts.ManagedByLabelKey: consts.OperatorName,
		consts.CreatedByLabelKey: consts.OperatorName,
	}
}

func (kr *KubeturboRequest) RestartDeployment(dep *appsv1.Deployment) (err error) {
	deployment := &appsv1.Deployment{}
	err = kr.Client.Get(kr.Context, client.ObjectKeyFromObject(dep), deployment)
	if err != nil {
		return
	}
	podAnnotations := deployment.Spec.Template.ObjectMeta.Annotations
	if podAnnotations == nil {
		podAnnotations = make(map[string]string)
	}
	podAnnotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	err = kr.Update(deployment)
	return
}
