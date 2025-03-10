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

package runnable

import (
	"context"
	"fmt"
	"os"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// CRDCheck is a custom Runnable for post-start checks
type CRDCheck struct {
	client.Client
	CRDName      string
	Recorder     record.EventRecorder
	CRDCheckDone *chan interface{}
}

var (
	logger = ctrl.Log.WithName("Post-start-check")
)

// Start is the method called after mgr.Start
func (r *CRDCheck) Start(ctx context.Context) error {
	logger.Info(fmt.Sprintf("Validation CRD %s...", r.CRDName))

	deployment := GetOperatorDeployment(ctx, r)
	if deployment == nil {
		logger.Info("The operator is not running in the pod mode")
	}

	// Check if the crd exists in the target cluster
	// Populate the error to cause the operator exit with an expectation
	if err := r.IsCrdExists(ctx); err != nil {
		if deployment != nil {
			r.Recorder.Event(deployment, "Warning", "CRDIssue", err.Error())
		}
		return err
	}

	// Check the existed crd satisfy the minimum requirements or not
	// Exit the operator with a message to ask the client to update their existing CRD
	if err := r.IsCrdUpToDate(ctx); err != nil {
		if deployment != nil {
			r.Recorder.Event(deployment, "Warning", "CRDIssue", err.Error())
		}
		return err
	}

	logger.Info(fmt.Sprintf("Validation CRD %s passed", r.CRDName))

	if r.CRDCheckDone != nil {
		close(*r.CRDCheckDone)
	}

	return nil
}

// Get the deployment object for the current pod container
func GetOperatorDeployment(ctx context.Context, c client.Client) *appsv1.Deployment {
	operator_pod_name := getOsEnv("POD_NAME")
	operator_pod_namespace := getOsEnv("WATCH_NAMESPACE")

	// if the pod name and the namespace is not set, then no need to find the deployment parent
	if operator_pod_name == nil || operator_pod_namespace == nil {
		return nil
	}

	// Get the running pod instance of the current program
	pod := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKey{
		Name:      *operator_pod_name,
		Namespace: *operator_pod_namespace,
	}, pod); err != nil {
		// If the pod of the running program is not found,
		// it means the program is running as a local program.
		// There's no need to get the parent deployment in this case
		return nil
	}

	// Get the owner reference
	deployment_name := ""
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			// Fetch the ReplicaSet to get the Deployment name
			rsName := owner.Name
			rs := &appsv1.ReplicaSet{}
			if err := c.Get(ctx, client.ObjectKey{
				Name:      rsName,
				Namespace: *operator_pod_namespace,
			}, rs); err != nil {
				logger.Error(err, "")
				return nil
			}
			for _, owner := range rs.OwnerReferences {
				if owner.Kind == "Deployment" {
					fmt.Printf("Running in deployment: %s\n", owner.Name)
					deployment_name = owner.Name
					break
				}
			}
		}
		if deployment_name != "" {
			break
		}
	}

	if deployment_name == "" {
		return nil
	}

	// Get the parent deployment object
	deployment := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{
		Name:      deployment_name,
		Namespace: *operator_pod_namespace,
	}, deployment); err != nil {
		return nil
	}

	return deployment
}

// Using the unconstructed object to detect if the CRD exists or not
func (r *CRDCheck) IsCrdExists(ctx context.Context) error {
	unstructuredObj := &unstructured.Unstructured{}
	unstructuredObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})

	// Fetch the target CRD as an unstructured object
	if err := r.Get(ctx, client.ObjectKey{Name: r.CRDName}, unstructuredObj); err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("CRD %s exists", r.CRDName))
	return nil
}

// Check if the crd is not in the old version
func (r *CRDCheck) IsCrdUpToDate(ctx context.Context) error {
	// Fetch the target as an CRD object
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Get(ctx, client.ObjectKey{Name: r.CRDName}, crd); err != nil {
		return err
	}

	// Check if the fetched CRD contains necessary fields to proceed
	if r.CRDName == constants.KubeturboCRDName {
		// Implementation of the Go-based operator using Operator SDK.
		// Enhanced CRD definitions to improve overall readability and usability.
		// 'controller-gen.kubebuilder.io/version' annotation in the CRD is the key
		// factor to distinguish the older CRD verses the new one
		// The following section is to verify if the existing Kubeturbo CRD satisfy the requirements.
		foundAnnotation := false
		for k := range crd.Annotations {
			if k == constants.ControlGenAnnotation {
				foundAnnotation = true
				break
			}
		}
		if !foundAnnotation {
			return fmt.Errorf("since 8.14.3, kubeturbo operator has moved from helm operator to go based operator. Please refer to https://ibm.biz/KubeturboCRD to install and upgrade to the latest CRD")
		}
	}

	logger.Info(fmt.Sprintf("CRD %s meets the minimum version requirement", r.CRDName))
	return nil
}

// Get global variable defined in OS
func getOsEnv(field string) *string {
	val, found := os.LookupEnv(field)
	if !found {
		return nil
	}
	return &val
}
