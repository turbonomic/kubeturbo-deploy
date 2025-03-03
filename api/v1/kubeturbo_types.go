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

package v1

import (
	"fmt"
	"reflect"
	"strings"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RoleTypeReadOnly     string = "turbo-cluster-reader"
	RoleTypeAdmin        string = "turbo-cluster-admin"
	RoleTypeClusterAdmin string = "cluster-admin"
	DefaultVersion       string = "VERSION"
	DefaultAnnotationKey string = "kubeturbo.io/controllable"
	DefaultAnnotationVal string = "false"
)

var (
	defaultKtVersion       = ""
	defaultSysWlNsPatterns = []string{"kube-.*", "openshift-.*", "cattle.*"}
)

// NB - if a block is marked as omitempty, it must be given a default value in order for inner fields to be populated with their defaults
// a single inner field value will suffice

// KubeturboSpec defines the desired state of Kubeturbo
type KubeturboSpec struct {
	// You can use this configuration to define how daemon pods are identified.
	// Note if you do not enable daemonPodDetectors, the default is to identify all pods running as kind = daemonSet
	// Any entry for daemonPodDetectors would overwrite default. Recommend you do not use this parameter.
	// +kubebuilder:default={}
	DaemonPodDetectors DaemonPodDetectors `json:"daemonPodDetectors,omitempty"` // no default
	// The annotationWhitelist allows users to define regular expressions to allow kubeturbo to collect
	// matching annotations for the specified entity type. By default, no annotations are collected.
	// These regular expressions accept the RE2 syntax (except for \C) as defined here: https://github.com/google/re2/wiki/Syntax
	AnnotationWhitelist AnnotationWhitelist `json:"annotationWhitelist,omitempty"` // no default
	// +kubebuilder:default={"kubeturbo.io/controllable":"false"}
	Annotations map[string]string `json:"annotations,omitempty"` // default: kubeturbo.io/controllable: "false"

	// Specify 'turbo-cluster-reader' or 'turbo-cluster-admin' as role name instead of the default using
	// the 'cluster-admin' role. A cluster role with this name will be created during deployment
	// If using a role name other than the pre-defined role names, cluster role will not be created. This role should be
	// existing in the cluster and should have the necessary permissions required for kubeturbo to work accurately.
	// +kubebuilder:default=cluster-admin
	// +kubebuilder:validation:Pattern="^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?::[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$"
	RoleName string `json:"roleName,omitempty"`

	// The name of cluster role binding. Default is turbo-all-binding. If role binding is updated from an existing kubeturbo instance,
	// the operator will not delete the existing role binding in the clsuter. Therefore, the user may want to manually delete the old
	// clusterrolebinding from the cluster so that the service account is no longer tied to the previous role binding.
	// +kubebuilder:default=turbo-all-binding
	RoleBinding string `json:"roleBinding,omitempty"` // default: "turbo-all-binding"

	// The name of the service account name. Default is turbo-user
	// +kubebuilder:default=turbo-user
	ServiceAccountName string `json:"serviceAccountName,omitempty"` // default: "turbo-user"

	// Kubeturbo replicaCount
	ReplicaCount *int32 `json:"replicaCount,omitempty"` // default: 1
	// Kubeturbo image details for deployments outside of RH Operator Hub
	// +kubebuilder:default={repository:icr.io/cpopen/turbonomic/kubeturbo, pullPolicy:IfNotPresent}
	Image KubeturboImage `json:"image,omitempty"`
	// Configuration for Turbo Server
	// +kubebuilder:default={turboServer:"https://Turbo_server_URL"}
	ServerMeta KubeturboServerMeta `json:"serverMeta,omitempty"`
	// Credentials to register probe with Turbo Server
	// +kubebuilder:default={turbonomicCredentialsSecretName:turbonomic-credentials}
	RestAPIConfig KubeturboRestAPIConfig `json:"restAPIConfig,omitempty"`
	// Configurations to register probe with Turbo Server
	// +kubebuilder:default={registrationTimeoutSec:300, restartOnRegistrationTimeout:true}
	SdkProtocolConfig KubeturboSdkProtocolConfig `json:"sdkProtocolConfig,omitempty"`
	// Enable or disable features
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
	// Create HA placement policy for Node to Hypervisor by node role. Master is default
	// +kubebuilder:default={nodeRoles:"\"master\""}
	HANodeConfig KubeturboHANodeConfig `json:"HANodeConfig,omitempty"`
	// Optional target configuration
	TargetConfig KubeturboTargetConfig `json:"targetConfig,omitempty"`
	// Kubeturbo command line arguments
	// +kubebuilder:default={logginglevel:2}
	Args KubeturboArgs `json:"args,omitempty"`
	// Kubeturbo resource configuration
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Optional logging level configuration. Changing this value does not require restart of Kubeturbo but takes about 1 minute to take effect
	// +kubebuilder:default={level:2}
	Logging Logging `json:"logging,omitempty"`

	// Optional node pool configuration. Changing this value does not require restart of Kubeturbo but takes about 1 minute to take effect
	// +kubebuilder:default={min:1, max: 1000}
	NodePoolSize NodePoolSize `json:"nodePoolSize,omitempty"`

	// Cluster Role rules for ORM owners. It's required when using ORM with ClusterRole 'turbo-cluster-admin'. It's recommended to use ORM with ClusterRole 'cluster-admin'
	OrmOwners OrmOwners `json:"ormOwners,omitempty"`

	// Flag system workloads such as those defined in kube-system, openshift-system, etc. Kubeturbo will not generate actions for workloads that match the supplied patterns
	// +kubebuilder:default={namespacePatterns:{kube-.*, openshift-.*, cattle.*}}
	SystemWorkloadDetectors SystemWorkloadDetectors `json:"systemWorkloadDetectors,omitempty"`

	// Identity operator-controlled workloads by name or namespace using regular expressions
	ExclusionDetectors ExclusionDetectors `json:"exclusionDetectors,omitempty"`

	// WireMock mode configuration
	// +kubebuilder:default={enabled:false, url: "wiremock:8080"}
	Wiremock Wiremock `json:"wiremock,omitempty"`

	// Discovery-related configurations
	// +kubebuilder:default={chunkSendDelayMillis: 0, numObjectsPerChunk: 5000}
	Discovery Discovery `json:"discovery,omitempty"`

	// Specify one or more kubeturbo pod scheduling constraints in the cluster.
	// See https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/ for examples on nodeSelector, affinity, tolerations
	KubeturboPodScheduling KubeturboPodScheduling `json:"kubeturboPodScheduling,omitempty"`
}

type KubeturboPodScheduling struct {
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty" protobuf:"bytes,18,opt,name=affinity"`
	// The pod this Toleration is attached to tolerates any taint that matches
	// the triple <key,value,effect> using the matching operator <operator>.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
}

type KubeturboImage struct {
	// Container repository
	// +kubebuilder:default=icr.io/cpopen/turbonomic/kubeturbo
	Repository string `json:"repository,omitempty"` // default: icr.io/cpopen/turbonomic/kubeturbo
	// Kubeturbo container image tag
	Tag *string `json:"tag,omitempty"` // no default
	// Busybox repository. default is busybox. This is overridden by cpufreqgetterRepository
	BusyboxRepository *string `json:"busyboxRepository,omitempty"` // no default
	// Repository used to get node cpufrequency.
	CpufreqgetterRepository *string `json:"cpufreqgetterRepository,omitempty"` // no default
	// +kubebuilder:default=IfNotPresent
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"` // default: IfNotPresent
	// Define the secret used to authenticate to the container image registry
	ImagePullSecret *string `json:"imagePullSecret,omitempty"` // no default
}

type KubeturboServerMeta struct {
	// Turbo Server major version
	Version *string `json:"version,omitempty"` // no default
	// URL for Turbo Server endpoint
	// +kubebuilder:default="https://Turbo_server_URL"
	TurboServer string `json:"turboServer,omitempty"` // default="https://Turbo_server_URL
	// Proxy server address
	Proxy *string `json:"proxy,omitempty"` // no default
}

type KubeturboRestAPIConfig struct {
	// Name of k8s secret that contains the turbo credentials
	// +kubebuilder:default=turbonomic-credentials
	TurbonomicCredentialsSecretName string `json:"turbonomicCredentialsSecretName,omitempty"` // default: "turbonomic-credentials"
	// Turbo admin user id
	OpsManagerUserName *string `json:"opsManagerUserName,omitempty"` // default: "Turbo_username" let's not add default to CRD
	// Turbo admin user password
	OpsManagerPassword *string `json:"opsManagerPassword,omitempty"` // default: "Turbo_password"
}

type KubeturboSdkProtocolConfig struct {
	// Time in seconds to wait for registration response from the Turbo Server
	// +kubebuilder:default=300
	RegistrationTimeoutSec *int `json:"registrationTimeoutSec,omitempty"` // default: 300
	// Restart probe container on registration timeout
	// +kubebuilder:default=true
	RestartOnRegistrationTimeout *bool `json:"restartOnRegistrationTimeout,omitempty"` // default: true
}

type KubeturboHANodeConfig struct {
	// Node role names
	// +kubebuilder:default="\"master\""
	NodeRoles string `json:"nodeRoles,omitempty"` // default: "\"master\""
}

type KubeturboTargetConfig struct {
	TargetName *string `json:"targetName,omitempty"` // no default
	// TargetType *string `json:"targetType,omitempty"` // no default
}

type KubeturboArgs struct {
	// Define logging level, default is info = 2
	// +kubebuilder:default=2
	Logginglevel *int `json:"logginglevel,omitempty"` // default: 2
	// Identify if kubelet requires https
	// +kubebuilder:default=true
	Kubelethttps *bool `json:"kubelethttps,omitempty"` // default: true
	// Identify kubelet port
	// +kubebuilder:default=10250
	Kubeletport *int `json:"kubeletport,omitempty"` // default: 10250
	// Allow kubeturbo to execute actions in OCP
	Sccsupport              *string `json:"sccsupport,omitempty"`              // no default
	ReadinessRetryThreshold *int32  `json:"readinessRetryThreshold,omitempty"` // no default (60 in kt pod)
	// Allow kubeturbo to reschedule pods with volumes attached
	FailVolumePodMoves *bool `json:"failVolumePodMoves,omitempty"` // no default (true in kt pod)
	// Do not run busybox on these nodes to discover the cpu frequency with k8s 1.18 and later, default is either of kubernetes.io/os=windows or beta.kubernetes.io/os=windows present as node label
	BusyboxExcludeNodeLabels *string `json:"busyboxExcludeNodeLabels,omitempty"` // no default, comma separated list of key=value node label pairs
	// Identify if using uuid or ip for stitching
	// +kubebuilder:default=true
	Stitchuuid *bool `json:"stitchuuid,omitempty"` // default: true
	// +kubebuilder:default=false
	Pre16K8sVersion *bool `json:"pre16k8sVersion,omitempty"` // default: false; CANDIDATE FOR REMOVAL
	// Identify if cleanup the resources created for scc impersonation, default is true
	// +kubebuilder:default=true
	CleanupSccImpersonationResources *bool `json:"cleanupSccImpersonationResources,omitempty"` // default: true
	// Skip creating the resources for scc impersonation
	// +kubebuilder:default=false
	SkipCreatingSccImpersonationResources *bool `json:"skipCreatingSccImpersonationResources,omitempty"`
	// The email to be used to push changes to git with ArgoCD integration
	GitEmail *string `json:"gitEmail,omitempty"` // no default
	// The username to be used to push changes to git with ArgoCD integration
	GitUsername *string `json:"gitUsername,omitempty"` // no default
	// The name of the secret which holds the git credentials to be used with ArgoCD integration
	GitSecretName *string `json:"gitSecretName,omitempty"` // no default
	// The namespace of the secret which holds the git credentials to be used with ArgoCD integration
	GitSecretNamespace *string `json:"gitSecretNamespace,omitempty"` // no default
	// The commit mode that should be used for git action executions with ArgoCD Integration. One of request or direct. Defaults to direct.
	GitCommitMode *string `json:"gitCommitMode,omitempty"` // no default
	// The IBM cloud satellite location provider, it only support azure as of today
	SatelliteLocationProvider *string `json:"satelliteLocationProvider,omitempty"`
	// The discovery interval in seconds
	// +kubebuilder:default=600
	DiscoveryIntervalSec *int `json:"discoveryIntervalSec,omitempty"`
	// The discovery interval in seconds to collect additional resource usage data samples from kubelet. This should be no smaller than 10 seconds.
	// +kubebuilder:default=60
	DiscoverySampleIntervalSec *int `json:"discoverySampleIntervalSec,omitempty"`
	// The number of resource usage data samples to be collected from kubelet in each full discovery cycle. This should be no larger than 60.
	// +kubebuilder:default=10
	DiscoverySamples *int `json:"discoverySamples,omitempty"`
	// The discovery timeout in seconds for each discovery worker. Default value is 180 seconds
	// +kubebuilder:default=180
	DiscoveryTimeoutSec *int `json:"discoveryTimeoutSec,omitempty"`
	// The garbage collection interval in minutes for potentially leaked pods due to failed actions and kubeturbo restarts. Default value is 10 minutes
	// +kubebuilder:default=10
	GarbageCollectionIntervalMin *int `json:"garbageCollectionIntervalMin,omitempty"`
	// The number of discovery workers. Default is 10
	// +kubebuilder:default=10
	DiscoveryWorkers *int `json:"discoveryWorkers,omitempty"`
}

type DaemonPodDetectors struct {
	PodNamePatterns   []string `json:"podNamePatterns,omitempty"`
	NamespacePatterns []string `json:"namespacePatterns,omitempty"`
}

type AnnotationWhitelist struct {
	ContainerSpec      *string `json:"containerSpec,omitempty"`
	Namespace          *string `json:"namespace,omitempty"`
	WorkloadController *string `json:"workloadController,omitempty"`
}

type Logging struct {
	// Define logging level
	// +kubebuilder:default=2
	Level *int `json:"level,omitempty"`
}

type NodePoolSize struct {
	// minimum number of nodes allowed in the node pool
	// +kubebuilder:default=1
	Min *int `json:"min,omitempty"`
	// maximum number of nodes allowed in the node pool
	// +kubebuilder:default=1000
	Max *int `json:"max,omitempty"`
}

type OrmOwners struct {
	// API group for ORM owners
	ApiGroup []string `json:"apiGroup,omitempty"`
	// resources for ORM owners
	Resources []string `json:"resources,omitempty"`
}

type SystemWorkloadDetectors struct {
	// A list of regular expressions that match the namespace names for system workloads
	// +kubebuilder:default={kube-.*, openshift-.*, cattle.*}
	NamespacePatterns []string `json:"namespacePatterns,omitempty"`
}

type ExclusionDetectors struct {
	// A list of regular expressions representing operator-controlled Workload Controllers. Workload Controllers that match the supplied expression will not have actions generated against them.
	OperatorControlledWorkloadsPatterns []string `json:"operatorControlledWorkloadsPatterns,omitempty"`
	// A list of regular expressions representing namespaces containing operator-controlled Workload Controllers. Workload Controllers deployed within the matching namespaces will not have actions generated against them.
	OperatorControlledNamespacePatterns []string `json:"operatorControlledNamespacePatterns,omitempty"`
}

type Wiremock struct {
	// Enable WireMock mode
	// +kubebuilder:default=false
	Enabled *bool `json:"enabled,omitempty"`
	// WireMock service URL
	// +kubebuilder:default="wiremock:8080"
	URL *string `json:"url,omitempty"`
}

type Discovery struct {
	// time delay (in milliseconds) between transmissions of chunked discovery data
	// +kubebuilder:default=0
	ChunkSendDelayMillis *int32 `json:"chunkSendDelayMillis,omitempty"`
	// Desired size (in number of DTOs) of discovery data chunks (default = 5,000)
	// +kubebuilder:default=5000
	NumObjectsPerChunk *int32 `json:"numObjectsPerChunk,omitempty"`
}

// +kubebuilder:pruning:PreserveUnknownFields
// KubeturboStatus defines the observed state of Kubeturbo
type KubeturboStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Timestamp of the last sync up
	LastUpdatedTimestamp string `json:"lastUpdatedTimestamp,omitempty"`
	// Hash of the constructed turbo.config file
	ConfigHash string `json:"configHash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=kubeturbos,shortName=kt
//+kubebuilder:subresource:status
//+kubebuilder:storageversion

// Kubeturbo is the Schema for the kubeturbos API
type Kubeturbo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:default={}
	Spec   KubeturboSpec   `json:"spec"`
	Status KubeturboStatus `json:"status,omitempty"`
}

func (kt *Kubeturbo) SetSpecDefault() error {
	var err error

	// If CR doesn't specify a version then use the DEFAULT_KUBETURBO_VERSION
	// as Kubeturbo's version tag. This behavior ensures the Kubeturbo pod will
	// always up-to-date when the operator bumping up its version. Won't affect
	// the scenario if the client want to use a fixed version that specified in
	// the CR.
	if defaultKtVersion == "" {
		defaultKtVersion, err = utils.GetDefaultKubeturboVersion()
		if err != nil {
			return err
		}
	}

	// Patch default version if the value is not specified
	if kt.Spec.Image.Tag == nil || *kt.Spec.Image.Tag == DefaultVersion {
		kt.Spec.Image.Tag = &defaultKtVersion
	}
	if kt.Spec.ServerMeta.Version == nil || *kt.Spec.ServerMeta.Version == DefaultVersion {
		kt.Spec.ServerMeta.Version = &defaultKtVersion
	}

	// Patch default annotations if the value is not specified
	if _, ok := kt.Spec.Annotations[DefaultAnnotationKey]; !ok {
		kt.Spec.Annotations = map[string]string{DefaultAnnotationKey: DefaultAnnotationVal}
	}

	// Patch default namespace patterns for SystemWorkloadDetectors if not specified
	if kt.Spec.SystemWorkloadDetectors.NamespacePatterns == nil {
		kt.Spec.SystemWorkloadDetectors.NamespacePatterns = defaultSysWlNsPatterns
	}

	return kt.VerifySubfields()
}

// Verify if the fetched Kubeturbo type contains all necessary fields
func (kt *Kubeturbo) VerifySubfields() error {
	// Following are the fields that cause the Kubeturbo pod unable to launch
	// Pause the reconcilation loop if any of the field is missing
	checkList := []string{
		"Spec.RoleName",
		"Spec.RoleBinding",
		"Spec.ServiceAccountName",
		"Spec.Image.Repository",
		"Spec.ServerMeta.TurboServer",
		"Spec.RestAPIConfig.TurbonomicCredentialsSecretName",
		"Spec.HANodeConfig.NodeRoles",
	}

	errorMessages := []string{}
	val := reflect.ValueOf(kt)

	// Ensure the input is a struct or a pointer to a struct
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}

	var scanFields func(reflect.Value, string)
	scanFields = func(v reflect.Value, parent string) {
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			fieldValue := v.Field(i)

			// Construct the full field name
			fieldName := field.Name
			if parent != "" {
				fieldName = parent + "." + fieldName
			}

			// Skip unexported fields
			if !fieldValue.CanInterface() {
				continue
			}

			if utils.StringInSlice(fieldName, checkList) {
				// Check if the field is a non-pointer and has a zero value
				if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
					errorMessages = append(errorMessages, fieldName)
				} else if fieldValue.Kind() == reflect.String && fieldValue.String() == "" {
					errorMessages = append(errorMessages, fieldName)
				}
			}

			// Recursively check nested structs
			if fieldValue.Kind() == reflect.Struct {
				scanFields(fieldValue, fieldName)
			}
		}
	}
	scanFields(val, "")

	// Summarize errors
	if len(errorMessages) > 0 {
		return fmt.Errorf("stopping reconciliation for Kubeturbo CR due to missing critical field(s): %s. Please review your CR and ensure the latest CRD is applied before proceeding", strings.Join(errorMessages, ", "))
	}
	return nil
}

//+kubebuilder:object:root=true

// KubeturboList contains a list of Kubeturbo
type KubeturboList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kubeturbo `json:"items"`
}

type ResourceRequirements struct {
	Limits   map[corev1.ResourceName]resource.Quantity `json:"limits,omitempty"`
	Requests map[corev1.ResourceName]resource.Quantity `json:"requests,omitempty"`
}

func (rr ResourceRequirements) Internalize() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits:   rr.Limits,
		Requests: rr.Requests,
	}
}

func init() {
	SchemeBuilder.Register(&Kubeturbo{}, &KubeturboList{})
}
