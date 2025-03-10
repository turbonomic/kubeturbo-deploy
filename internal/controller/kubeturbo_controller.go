/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kubeturbosv1 "github.ibm.com/turbonomic/kubeturbo-deploy/api/v1"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/api/kubeturbo"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/constants"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/reconcile"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// KubeturboReconciler reconciles a Kubeturbo object
type KubeturboReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	PostCheckDone *chan interface{}
}

//+kubebuilder:rbac:groups=charts.helm.k8s.io,resources=kubeturbos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=charts.helm.k8s.io,resources=kubeturbos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=charts.helm.k8s.io,resources=kubeturbos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Kubeturbo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *KubeturboReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Block processing CR until all pre-checks completed
	if r.PostCheckDone != nil {
		<-*r.PostCheckDone
	}

	logger := log.FromContext(ctx)

	logger.Info("Reconciling...")
	defer logger.Info("Reconcile complete")

	var kt kubeturbosv1.Kubeturbo
	err := r.Get(ctx, req.NamespacedName, &kt)

	// if failed to load to the Kubeturbo object
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.DoNotRequeue().Get() // Custom resource has been deleted
		}
		return reconcile.RequeueOnError(err).Get()
	}

	// When the operator accidentally hit CR that is processed by the old CRD,
	// there might be some fields missed default values, so we need to scan and
	// patch fields default values that aren't bring up by the CR creation.
	err = kt.SetSpecDefault()
	if err != nil {
		logger.Error(err, "")
		return reconcile.DoNotRequeue().Get()
	}

	// Ensures only the finalizer added by this opeartor is used by the CR
	needToUpdateCR := false
	for _, f := range kt.GetFinalizers() {
		if f != constants.KubeturboFinalizer {
			controllerutil.RemoveFinalizer(&kt, f)
			needToUpdateCR = true
		}
	}
	if !controllerutil.ContainsFinalizer(&kt, constants.KubeturboFinalizer) {
		logger.Info("Patching finalizer to CR")
		controllerutil.AddFinalizer(&kt, constants.KubeturboFinalizer)
		needToUpdateCR = true
	}

	// Check if the Kubeturbo instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isKubeturboMarkedToBeDeleted := kt.GetDeletionTimestamp() != nil
	if isKubeturboMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(&kt, constants.KubeturboFinalizer) {
			// Remove kubeturboFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(&kt, constants.KubeturboFinalizer)
			err := r.Update(ctx, &kt)
			if err != nil {
				return reconcile.RequeueOnError(err).Get()
			}

			if kt.Spec.Args.CleanupSccImpersonationResources != nil && *kt.Spec.Args.CleanupSccImpersonationResources {
				//If cleanup scc flag is true, we need to wait for scc resources, pod to delete before removing
				//service account.
				r.waitForPodDeletion(ctx, req.NamespacedName, &kt)
			}
			//clean up ClusterResources, serviceaccount
			if err = kubeturbo.Teardown(ctx, r.Client, r.Scheme, &kt); err != nil {
				return reconcile.RequeueOnError(err).Get()
			}
		}
		return reconcile.DoNotRequeue().Get()
	}

	// Patch the existing CR with a specific finalizer, if needed
	if needToUpdateCR {
		logger.Info("Updating CR with finalizer")
		jsonPatch, err := json.Marshal([]map[string]interface{}{
			{
				"op":    "replace",
				"path":  "/metadata/finalizers",
				"value": kt.GetFinalizers(),
			},
		})

		// Patch only necessary field to minimize impact to reconcile loop
		if err != nil {
			return reconcile.RequeueOnError(err).Get()
		} else if err := r.Patch(ctx, &kt, client.RawPatch(types.JSONPatchType, jsonPatch)); err != nil {
			return reconcile.RequeueOnError(err).Get()
		}

		// Do not requeue, since patching CR will trigger another reconcile cycle
		return reconcile.DoNotRequeue().Get()
	}

	// Only CR that patches with correct finalizer will reach to the reconcile cycle
	if err := kubeturbo.Reconcile(ctx, r.Client, r.Scheme, &kt); err != nil {
		// if race condition happened or on resource deletion, delay the requeue
		if errors.IsConflict(err) || err == constants.ErrRequeueOnDeletion {
			logger.Info(fmt.Sprintf("Warning: To avoid race condition, retry reconciliation process in %ds", constants.RequeueDelaySeconds))
			return reconcile.RequeueAfter(time.Duration(constants.RequeueDelaySeconds * time.Second)).Get()
		}
		return reconcile.RequeueOnError(err).Get()
	}

	// if not error, terminate the current reconcile cycle
	return reconcile.DoNotRequeue().Get()
}

func (r *KubeturboReconciler) waitForPodDeletion(ctx context.Context, namespace types.NamespacedName, kt client.Object) {
	logger := log.FromContext(ctx)
	if err := wait.PollUntilContextTimeout(ctx, time.Second, constants.TimeoutInSeconds*time.Second, false, func(ctx context.Context) (bool, error) {
		podList := r.GetPodByDeployment(ctx, kt.GetName(), namespace.Namespace)
		if len(podList.Items) == 0 {
			return true, nil
		}
		return false, nil
	}); err != nil {
		logger.Error(err, fmt.Sprintf("pod for cr: %s not deleted within timeout %d seconds", kt.GetName(), constants.TimeoutInSeconds))
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *KubeturboReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeturbosv1.Kubeturbo{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		// won't work for cluster-level resources
		Complete(r)
}

func (r *KubeturboReconciler) GetPodByDeployment(ctx context.Context, deployName string, namespace string) corev1.PodList {
	logger := log.FromContext(ctx)
	podList := corev1.PodList{}
	err := r.List(ctx, &podList, &client.ListOptions{
		Namespace: namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/name": deployName,
		}),
	})
	if err != nil {
		logger.Error(err, "")
	}
	return podList
}
