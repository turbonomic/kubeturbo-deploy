package kubeturbo

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeturbosv1 "github.ibm.com/turbonomic/kubeturbo-deploy/api/v1"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/constants"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
)

const (
	serviceAccountFinalizer = "helm.k8s.io/finalizer"
)

type kubeturbo struct {
	*KubeturboRequest
	spec   kubeturbosv1.KubeturboSpec
	logger logr.Logger
}

type block = utils.Block

func Reconcile(ctx context.Context, client client.Client, scheme *runtime.Scheme, ktV1 *kubeturbosv1.Kubeturbo) error {
	logger := log.FromContext(ctx).WithName("TearUp-cycle")
	kr := NewKubeturboRequest(client, ctx, scheme, ktV1)
	kt := kubeturbo{KubeturboRequest: kr, spec: kr.Cr.Spec, logger: logger}
	return kt.reconcileKubeTurbo()
}

func Teardown(ctx context.Context, client client.Client, scheme *runtime.Scheme, ktV1 *kubeturbosv1.Kubeturbo) error {
	logger := log.FromContext(ctx).WithName("TearDown-cycle")
	kr := NewKubeturboRequest(client, ctx, scheme, ktV1)
	kt := kubeturbo{KubeturboRequest: kr, spec: kr.Cr.Spec, logger: logger}
	return kt.cleanUpClusterResources()
}

func (kt *kubeturbo) reconcileKubeTurbo() error {
	return utils.ReturnOnError(
		kt.createOrUpdateConfigMap,
		kt.createOrUpdateServiceAccount,
		kt.createOrUpdateClusterRole,
		kt.createOrUpdateClusterRoleBinding,
		kt.createOrUpdateDeployment,
		kt.updateClusterResource,
	)
}

func (kt *kubeturbo) deployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: kt.Name(), Namespace: kt.Namespace()}}
}

func (kt *kubeturbo) createOrUpdateDeployment() error {
	dep := kt.deployment()
	kt.SetControllerReference(dep)

	// The Kubeturbo pod need to restart to loop in config updates
	oldConfigMapHash := kt.Cr.Status.ConfigHash
	newConfigMapHash, hashErr := kt.getKubeturboConfigHash()
	if hashErr != nil {
		return hashErr
	}

	// race condition: When deploy get deleted by the previous request while
	// the second reconcile cycle arrived between the deployment get deleted
	// and CR status updates
	if oldConfigMapHash != "" && oldConfigMapHash != newConfigMapHash {
		// update CR hash to prevent infinity loop
		kt.updateClusterResource()

		kt.logger.Info("Kubeturbo deploy needs to restart to pick up changes")
		if err := kt.DeleteIfExists(kt.deployment()); err != nil {
			return err
		}
		return constants.ErrRequeueOnDeletion
	}

	_, err := kt.CreateOrUpdate(dep, func() error {
		return kt.mutateDeployment(dep)
	})
	return err
}

func (kt *kubeturbo) mutateDeployment(dep *appsv1.Deployment) error {
	labels := kt.labels()

	// If customer upgrade from helm operator to go-based operator, the labels under selector will be different.
	// Since selector in a deployment is immutable, we will need to delete the deployment and recreate it.
	if dep.Spec.Selector != nil && !reflect.DeepEqual(labels, dep.Spec.Selector.MatchLabels) {
		if err := kt.DeleteIfExists(kt.deployment()); err != nil {
			return err
		}
		return constants.ErrRequeueOnDeletion
	}

	metadata := &dep.ObjectMeta
	metadata.Labels = labels

	imagePullSecrets := make([]corev1.LocalObjectReference, 0, 1)
	if kt.spec.Image.ImagePullSecret != nil {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{
			Name: *kt.spec.Image.ImagePullSecret,
		})
	}

	imagePullPolicy := corev1.PullIfNotPresent
	if kt.spec.Image.PullPolicy != nil {
		imagePullPolicy = *kt.spec.Image.PullPolicy
	}

	var resourceRequirements corev1.ResourceRequirements
	if kt.spec.Resources != nil {
		resourceRequirements = kt.spec.Resources.Internalize()
	}

	dep.Spec = appsv1.DeploymentSpec{
		Replicas: kt.spec.ReplicaCount,
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: kt.spec.Annotations,
				Labels:      labels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: kt.serviceAccountName(),
				ImagePullSecrets:   imagePullSecrets,
				RestartPolicy:      corev1.RestartPolicyAlways,
				NodeSelector:       kt.spec.KubeturboPodScheduling.NodeSelector,
				Affinity:           kt.spec.KubeturboPodScheduling.Affinity,
				Tolerations:        kt.spec.KubeturboPodScheduling.Tolerations,
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: utils.AsPtr(true),
				},
				Containers: []corev1.Container{
					{
						Name: constants.KubeturboContainerName,
						Env: []corev1.EnvVar{
							{
								Name: "KUBETURBO_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
						},
						Image:           fmt.Sprint(kt.spec.Image.Repository, ":", *kt.spec.Image.Tag),
						ImagePullPolicy: imagePullPolicy,
						Args:            kt.containerArgs(),
						SecurityContext: &corev1.SecurityContext{
							Privileged:               utils.AsPtr(false),
							AllowPrivilegeEscalation: utils.AsPtr(false),
							ReadOnlyRootFilesystem:   utils.AsPtr(true),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Resources: resourceRequirements,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "turbo-volume",
								MountPath: "/etc/kubeturbo",
								ReadOnly:  true,
							},
							{
								Name:      "turbonomic-credentials-volume",
								MountPath: "/etc/turbonomic-credentials",
								ReadOnly:  true,
							},
							{
								Name:      "varlog",
								MountPath: "/var/log",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "turbo-volume",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: kt.configMap().Name,
								},
							},
						},
					},
					{
						Name: "turbonomic-credentials-volume",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName:  kt.spec.RestAPIConfig.TurbonomicCredentialsSecretName,
								Optional:    utils.AsPtr(true),
								DefaultMode: utils.AsPtr(int32(420)),
							},
						},
					},
					{
						Name: "varlog",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
	}

	return nil
}

func (kt *kubeturbo) containerArgs() []string {
	args := make([]string, 0, 25)

	ktArgs := kt.spec.Args

	args = append(args, "--turboconfig=/etc/kubeturbo/turbo.config")
	if ktArgs.Logginglevel != nil {
		args = append(args, fmt.Sprintf("--v=%d", *ktArgs.Logginglevel))
	}
	if ktArgs.Kubelethttps != nil {
		args = append(args, fmt.Sprintf("--kubelet-https=%t", *ktArgs.Kubelethttps))
	}
	if ktArgs.Kubeletport != nil {
		args = append(args, fmt.Sprintf("--kubelet-port=%d", *ktArgs.Kubeletport))
	}
	if ktArgs.Sccsupport != nil {
		args = append(args, fmt.Sprint("--scc-support=", *ktArgs.Sccsupport))
	}
	if ktArgs.ReadinessRetryThreshold != nil {
		args = append(args, fmt.Sprint("--readiness-retry-threshold=", *ktArgs.ReadinessRetryThreshold))
	}
	if ktArgs.FailVolumePodMoves != nil {
		args = append(args, fmt.Sprint("--fail-volume-pod-moves=", *ktArgs.FailVolumePodMoves))
	}
	if kt.spec.Image.BusyboxRepository != nil {
		args = append(args, fmt.Sprint("--busybox-image=", *kt.spec.Image.BusyboxRepository))
	}
	if kt.spec.Image.ImagePullSecret != nil {
		args = append(args, fmt.Sprint("--busybox-image-pull-secret=", *kt.spec.Image.ImagePullSecret))
		args = append(args, fmt.Sprint("--cpufreqgetter-image-pull-secret=", *kt.spec.Image.ImagePullSecret))
	}
	if kt.spec.Image.CpufreqgetterRepository != nil {
		args = append(args, fmt.Sprint("--cpufreqgetter-image=", *kt.spec.Image.CpufreqgetterRepository))
	}
	if ktArgs.BusyboxExcludeNodeLabels != nil {
		args = append(args, fmt.Sprint("--cpufreq-job-exclude-node-labels=", *ktArgs.BusyboxExcludeNodeLabels))
	}
	if ktArgs.Stitchuuid != nil {
		args = append(args, fmt.Sprintf("--stitch-uuid=%t", *ktArgs.Stitchuuid))
	}
	if ktArgs.Pre16K8sVersion != nil && *ktArgs.Pre16K8sVersion {
		args = append(args, "--k8sVersion=1.5")
	}
	if ktArgs.CleanupSccImpersonationResources != nil {
		args = append(args, fmt.Sprintf("--cleanup-scc-impersonation-resources=%t", *ktArgs.CleanupSccImpersonationResources))
	}
	if ktArgs.SkipCreatingSccImpersonationResources != nil {
		args = append(args, fmt.Sprintf("--skip-creating-scc-impersonation-resources=%t", *ktArgs.SkipCreatingSccImpersonationResources))
	}
	if ktArgs.GitEmail != nil {
		args = append(args, fmt.Sprintf("--git-email=%s", *ktArgs.GitEmail))
	}
	if ktArgs.GitUsername != nil {
		args = append(args, fmt.Sprintf("--git-username=%s", *ktArgs.GitUsername))
	}
	if ktArgs.GitSecretName != nil {
		args = append(args, fmt.Sprintf("--git-secret-name=%s", *ktArgs.GitSecretName))
	}
	if ktArgs.GitSecretNamespace != nil {
		args = append(args, fmt.Sprintf("--git-secret-namespace=%s", *ktArgs.GitSecretNamespace))
	}
	if ktArgs.GitCommitMode != nil {
		args = append(args, fmt.Sprintf("--git-commit-mode=%s", *ktArgs.GitCommitMode))
	}
	if ktArgs.SatelliteLocationProvider != nil {
		args = append(args, fmt.Sprintf("--satellite-location-provider=%s", *ktArgs.SatelliteLocationProvider))
	}
	if ktArgs.DiscoveryIntervalSec != nil {
		args = append(args, fmt.Sprintf("--discovery-interval-sec=%d", *ktArgs.DiscoveryIntervalSec))
	}
	if ktArgs.DiscoverySampleIntervalSec != nil {
		args = append(args, fmt.Sprintf("--discovery-sample-interval=%d", *ktArgs.DiscoverySampleIntervalSec))
	}
	if ktArgs.DiscoverySamples != nil {
		args = append(args, fmt.Sprintf("--discovery-samples=%d", *ktArgs.DiscoverySamples))
	}
	if ktArgs.DiscoveryTimeoutSec != nil {
		args = append(args, fmt.Sprintf("--discovery-timeout-sec=%d", *ktArgs.DiscoveryTimeoutSec))
	}
	if ktArgs.GarbageCollectionIntervalMin != nil {
		args = append(args, fmt.Sprintf("--garbage-collection-interval=%d", *ktArgs.GarbageCollectionIntervalMin))
	}
	if ktArgs.DiscoveryWorkers != nil {
		args = append(args, fmt.Sprintf("--discovery-workers=%d", *ktArgs.DiscoveryWorkers))
	}
	return args
}

func (kt *kubeturbo) configMapName() string {
	return fmt.Sprint("turbo-config", "-", kt.Name())
}

func (kt *kubeturbo) configMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: kt.configMapName(), Namespace: kt.Namespace()}}
}

func (kt *kubeturbo) createOrUpdateConfigMap() error {
	cm := kt.configMap()
	kt.SetControllerReference(cm)
	_, err := kt.CreateOrUpdate(cm, func() error {
		return kt.mutateConfigMap(cm)
	})
	return err
}

func (kt *kubeturbo) mutateConfigMap(cm *corev1.ConfigMap) error {
	// kubeturbo config
	cByteString, cError := kt.buildKubeturboConfig()
	if cError != nil {
		return cError
	}

	// dynamic config
	dcByteString, dcError := kt.buildKubeturboDynamicConfig()
	if dcError != nil {
		return dcError
	}

	labels := kt.labels()
	cm.ObjectMeta.Labels = labels
	cm.Data = map[string]string{
		"turbo.config":            string(cByteString),
		"turbo-autoreload.config": string(dcByteString),
	}

	return nil
}

func (kt *kubeturbo) getKubeturboConfigHash() (string, error) {
	cByteString, err := kt.buildKubeturboConfig()
	if err != nil {
		return "", err
	}

	hash := fnv.New64()
	if _, err = hash.Write(cByteString); err != nil {
		return "", err
	}

	return fmt.Sprint(hash.Sum64()), nil
}

func (kt *kubeturbo) buildKubeturboConfig() ([]byte, error) {
	shortVersion := *kt.spec.ServerMeta.Version

	serverMeta := block{
		"version": shortVersion,
	}
	if kt.spec.ServerMeta.TurboServer != "" {
		serverMeta["turboServer"] = kt.spec.ServerMeta.TurboServer
	}
	if kt.spec.ServerMeta.Proxy != nil {
		serverMeta["proxy"] = *kt.spec.ServerMeta.Proxy
	}

	commConfig := block{"serverMeta": serverMeta}

	restApiConfig := block{}
	if kt.spec.RestAPIConfig.OpsManagerUserName != nil && kt.spec.RestAPIConfig.OpsManagerPassword != nil {
		if kt.spec.RestAPIConfig.OpsManagerUserName != nil {
			restApiConfig["opsManagerUserName"] = *kt.spec.RestAPIConfig.OpsManagerUserName
		}
		if kt.spec.RestAPIConfig.OpsManagerPassword != nil {
			restApiConfig["opsManagerPassword"] = *kt.spec.RestAPIConfig.OpsManagerPassword
		}
		commConfig["restAPIConfig"] = restApiConfig
	}

	sdkProtocolConfig := block{}
	if kt.spec.SdkProtocolConfig.RegistrationTimeoutSec != nil || kt.spec.SdkProtocolConfig.RestartOnRegistrationTimeout != nil {
		if kt.spec.SdkProtocolConfig.RegistrationTimeoutSec != nil {
			sdkProtocolConfig["registrationTimeoutSec"] = *kt.spec.SdkProtocolConfig.RegistrationTimeoutSec
		}
		if kt.spec.SdkProtocolConfig.RestartOnRegistrationTimeout != nil {
			sdkProtocolConfig["restartOnRegistrationTimeout"] = *kt.spec.SdkProtocolConfig.RestartOnRegistrationTimeout
		}
		commConfig["sdkProtocolConfig"] = sdkProtocolConfig
	}

	// convert HANodeConfig from string to slice to strip the quotation marks
	nodeRoles := strings.Split(kt.spec.HANodeConfig.NodeRoles, ",")
	for i, nr := range nodeRoles {
		if nr[0] == '"' && nr[len(nr)-1] == '"' {
			nodeRoles[i] = nr[1 : len(nr)-1]
		}
	}
	config := block{
		"communicationConfig": commConfig,
		"HANodeConfig": block{
			"roles": nodeRoles,
		},
	}

	if kt.spec.FeatureGates != nil && len(kt.spec.FeatureGates) > 0 {
		config["featureGates"] = kt.spec.FeatureGates
	}

	targetConfig := block{}
	if kt.spec.TargetConfig.TargetName != nil {
		targetConfig["targetName"] = *kt.spec.TargetConfig.TargetName
		config["targetConfig"] = targetConfig
	}

	hasAnnotationWhiteList := false
	annotationWhiteList := block{}
	if kt.spec.AnnotationWhitelist.ContainerSpec != nil {
		hasAnnotationWhiteList = true
		annotationWhiteList["containerSpec"] = *kt.spec.AnnotationWhitelist.ContainerSpec
	}
	if kt.spec.AnnotationWhitelist.Namespace != nil {
		hasAnnotationWhiteList = true
		annotationWhiteList["namespace"] = *kt.spec.AnnotationWhitelist.Namespace
	}
	if kt.spec.AnnotationWhitelist.WorkloadController != nil {
		hasAnnotationWhiteList = true
		annotationWhiteList["workloadController"] = *kt.spec.AnnotationWhitelist.WorkloadController
	}
	if hasAnnotationWhiteList {
		config["annotationWhiteList"] = annotationWhiteList
	}

	return json.MarshalIndent(config, "", "  ")
}

func (kt *kubeturbo) buildKubeturboDynamicConfig() ([]byte, error) {
	config := block{}

	if kt.spec.Logging.Level != nil {
		config["logging"] = *kt.spec.Logging.Level
	}

	nodePoolSizeConfig := block{}
	if kt.spec.NodePoolSize.Min != nil || kt.spec.NodePoolSize.Max != nil {
		if kt.spec.NodePoolSize.Min != nil {
			nodePoolSizeConfig["min"] = *kt.spec.NodePoolSize.Min
		}
		if kt.spec.NodePoolSize.Max != nil {
			nodePoolSizeConfig["mac"] = *kt.spec.NodePoolSize.Max
		}
		config["nodePoolSize"] = nodePoolSizeConfig
	}

	if kt.spec.SystemWorkloadDetectors.NamespacePatterns != nil {
		config["systemWorkloadDetectors"] = block{
			"namespacePatterns": kt.spec.SystemWorkloadDetectors.NamespacePatterns,
		}
	}

	exclusionDetectorsConfigs := block{}
	if kt.spec.ExclusionDetectors.OperatorControlledWorkloadsPatterns != nil || kt.spec.ExclusionDetectors.OperatorControlledNamespacePatterns != nil {
		if kt.spec.ExclusionDetectors.OperatorControlledWorkloadsPatterns != nil {
			exclusionDetectorsConfigs["operatorControlledWorkloadsPatterns"] = kt.spec.ExclusionDetectors.OperatorControlledWorkloadsPatterns
		}
		if kt.spec.ExclusionDetectors.OperatorControlledNamespacePatterns != nil {
			exclusionDetectorsConfigs["operatorControlledNamespacePatterns"] = kt.spec.ExclusionDetectors.OperatorControlledNamespacePatterns
		}
		config["exclusionDetectors"] = exclusionDetectorsConfigs
	}

	daemonPodDetectorsConfig := block{}
	if kt.spec.DaemonPodDetectors.NamespacePatterns != nil || kt.spec.DaemonPodDetectors.PodNamePatterns != nil {
		if kt.spec.DaemonPodDetectors.NamespacePatterns != nil {
			daemonPodDetectorsConfig["operatorControlledWorkloadsPatterns"] = kt.spec.DaemonPodDetectors.NamespacePatterns
		}
		if kt.spec.DaemonPodDetectors.PodNamePatterns != nil {
			daemonPodDetectorsConfig["operatorControlledNamespacePatterns"] = kt.spec.DaemonPodDetectors.PodNamePatterns
		}
		config["daemonPodDetectors"] = daemonPodDetectorsConfig
	}

	discoveryConfig := block{}
	if kt.spec.Discovery.ChunkSendDelayMillis != nil || kt.spec.Discovery.NumObjectsPerChunk != nil {
		if kt.spec.Discovery.ChunkSendDelayMillis != nil {
			discoveryConfig["chunkSendDelayMillis"] = *kt.spec.Discovery.ChunkSendDelayMillis
		}
		if kt.spec.Discovery.NumObjectsPerChunk != nil {
			discoveryConfig["numObjectsPerChunk"] = *kt.spec.Discovery.NumObjectsPerChunk
		}
		config["discovery"] = discoveryConfig
	}

	if kt.spec.Wiremock.Enabled != nil && kt.spec.Wiremock.URL != nil && *kt.spec.Wiremock.Enabled {
		config["wiremock"] = block{
			"enabled": *kt.spec.Wiremock.Enabled,
			"url":     *kt.spec.Wiremock.URL,
		}
	}

	return json.MarshalIndent(config, "", "  ")
}

func (kt *kubeturbo) serviceAccountName() string {
	return kt.spec.ServiceAccountName
}

func (kt *kubeturbo) serviceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: kt.serviceAccountName(), Namespace: kt.Namespace()}}
}

func (kt *kubeturbo) createOrUpdateServiceAccount() error {
	sa := kt.serviceAccount()
	kt.SetControllerReference(sa)
	_, err := kt.CreateOrUpdate(sa, func() error {
		return kt.mutateServiceAccount(sa, true)
	})
	return err
}

func (kt *kubeturbo) mutateServiceAccount(sa *corev1.ServiceAccount, addFinalizer bool) error {
	sa.ObjectMeta.Labels = kt.labels()
	if !addFinalizer {
		kt.logger.Info("Remove finalizer from service account.")
		sa.ObjectMeta.Finalizers = []string{}
	} else if !controllerutil.ContainsFinalizer(sa, serviceAccountFinalizer) {
		kt.logger.Info("Add finalizer to service account.")
		sa.ObjectMeta.Finalizers = []string{serviceAccountFinalizer}
	}
	return nil
}

func (kt *kubeturbo) clusterRoleName() string {
	roleName := kt.spec.RoleName
	if kt.spec.RoleName == kubeturbosv1.RoleTypeAdmin || kt.spec.RoleName == kubeturbosv1.RoleTypeReadOnly {
		roleName = kt.spec.RoleName + "-" + kt.Name() + "-" + kt.Namespace()
	}
	return roleName
}

func (kt *kubeturbo) clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: kt.clusterRoleName()}}
}

func (kt *kubeturbo) createOrUpdateClusterRole() error {

	// if roleName is cluster-admin or any custom names other than "turbo-cluster-admin" or "turbo-cluster-reader", don't override it
	if kt.spec.RoleName == kubeturbosv1.RoleTypeClusterAdmin || (kt.spec.RoleName != kubeturbosv1.RoleTypeAdmin && kt.spec.RoleName != kubeturbosv1.RoleTypeReadOnly) {
		return nil
	}

	cr := kt.clusterRole()

	_, err := kt.CreateOrUpdate(cr, func() error {
		return kt.mutateClusterRole(cr)
	})
	return err
}

func (kt *kubeturbo) mutateClusterRole(cr *rbacv1.ClusterRole) error {

	cr.Labels = kt.labels()

	// turbo-cluster-reader
	if kt.spec.RoleName == kubeturbosv1.RoleTypeReadOnly {
		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "apps", "app.k8s.io", "apps.openshift.io", "batch", "extensions", "turbonomic.com", "devops.turbonomic.io", "config.openshift.io"},
				Resources: []string{
					// ""
					"endpoints", "limitranges", "namespaces", "nodes", "persistentvolumeclaims", "persistentvolumes", "pods", "replicationcontrollers", "resourcequotas", "services",
					// "apps"
					"daemonsets", "deployments", "replicasets", "statefulsets",
					// "app.k8s.io"
					"applications",
					// "apps.openshift.io"
					"deploymentconfigs",
					// "batch"
					"jobs", "cronjobs",
					// "turbonomic.com"
					"operatorresourcemappings", "clusterversions",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"machine.openshift.io"},
				Resources: []string{"machines", "machinesets"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/spec", "nodes/stats", "nodes/metrics", "nodes/proxy"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"policy.turbonomic.io"},
				Resources: []string{"slohorizontalscales", "containerverticalscales", "policybindings"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
	} else if kt.spec.RoleName == kubeturbosv1.RoleTypeAdmin {
		// turbo-cluster-admin
		cr.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"", "batch"},
				Resources: []string{"pods", "jobs"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"", "apps", "apps.openshift.io", "extensions", "turbonomic.com", "devops.turbonomic.io", "redis.redis.opstreelabs.in", "charts.helm.k8s.io"},
				Resources: []string{"deployments", "replicasets", "replicationcontrollers", "statefulsets", "daemonsets", "deploymentconfigs", "resourcequotas", "operatorresourcemappings", "operatorresourcemappings/status", "redis", "xls"},
				Verbs:     []string{"get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"", "apps", "batch", "extensions", "policy", "app.k8s.io", "argoproj.io", "apiextensions.k8s.io", "config.openshift.io"},
				Resources: []string{"nodes", "services", "endpoints", "namespaces", "limitranges", "persistentvolumes", "persistentvolumeclaims", "poddisruptionbudget", "cronjobs", "applications", "customresourcedefinitions", "clusterversions"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"machine.openshift.io"},
				Resources: []string{"machines", "machinesets"},
				Verbs:     []string{"get", "list", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes/spec", "nodes/stats", "nodes/metrics", "nodes/proxy", "pods/log"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"policy.turbonomic.io"},
				Resources: []string{"slohorizontalscales", "containerverticalscales", "policybindings"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"security.openshift.io"},
				Resources: []string{"securitycontextconstraints"},
				Verbs:     []string{"list", "use"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"serviceaccounts"},
				Verbs:     []string{"get", "create", "delete", "impersonate"},
			},
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"},
				Verbs:     []string{"get", "create", "delete", "update"},
			},
		}
	}

	if kt.spec.OrmOwners.ApiGroup != nil && kt.spec.OrmOwners.Resources != nil {
		cr.Rules = append(cr.Rules, rbacv1.PolicyRule{
			APIGroups: kt.spec.OrmOwners.ApiGroup,
			Resources: kt.spec.OrmOwners.Resources,
			Verbs:     []string{"get", "list", "patch", "update", "watch"},
		})
	}

	return nil
}

func (kt *kubeturbo) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: kt.spec.RoleBinding + "-" + kt.Name() + "-" + kt.Namespace()}}
}

func (kt *kubeturbo) createOrUpdateClusterRoleBinding() error {
	crb := kt.clusterRoleBinding()
	// TODO - this doesn't work on cluster-level resources
	// kt.SetControllerReference(crb)
	_, err := kt.CreateOrUpdate(crb, func() error {
		return kt.mutateClusterRoleBinding(crb)
	})
	return err
}

func (kt *kubeturbo) mutateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding) error {
	// role ref cannot be updated in an existing role binding. Therefore,
	// if role name is updated in the CR, delete the existing role binding before creating it
	if crb.RoleRef.Name != "" && crb.RoleRef.Name != kt.clusterRoleName() {
		if err := kt.DeleteIfExists(kt.clusterRoleBinding()); err != nil {
			return err
		}
		return constants.ErrRequeueOnDeletion
	}
	crb.Labels = kt.labels()

	crb.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      kt.serviceAccount().Name,
			Namespace: kt.Namespace(),
		},
	}

	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     kt.clusterRoleName(),
		APIGroup: rbacv1.GroupName,
	}

	return nil
}

func (kt *kubeturbo) updateClusterResource() error {
	newConfigMapHash, hashErr := kt.getKubeturboConfigHash()
	if hashErr != nil {
		return hashErr
	}
	// update hash once the hash got changed
	oldConfigHash := kt.Cr.Status.ConfigHash
	if newConfigMapHash != oldConfigHash {
		kt.Cr.Status.LastUpdatedTimestamp = time.Now().Format(time.RFC3339)
		kt.Cr.Status.ConfigHash = newConfigMapHash
		return kt.UpdateStatus()
	}
	return nil
}

func (kt *kubeturbo) labels() map[string]string {
	return utils.NewMapBuilder[string, string]().
		PutAll(kt.ReleaseLabels()).
		Put(constants.NameLabelKey, kt.Name()).
		Put(constants.ComponentLabelKey, constants.KubeturboComponentType).
		Build()
}

func (kt *kubeturbo) cleanUpClusterResources() error {
	//remove finalizer from service account
	sa := kt.serviceAccount()
	_, err := kt.CreateOrUpdate(sa, func() error {
		return kt.mutateServiceAccount(sa, false)
	})
	if err != nil {
		return err
	}

	// delete cluster level objects created by the operator
	return utils.ReturnOnError(
		kt.cleanUpClusterRole,
		kt.cleanUpClusterRolebinding,
	)
}

// Delete all clusterRoles created by the CR
func (kt *kubeturbo) cleanUpClusterRole() error {
	// Since we do not generate ClusterRoles for the roles provided by the client,
	// the client's ClusterRole will not match the label selector that filters for
	// operator-managed roles. Therefore, distinguishing the current ClusterRole
	// in the CR is unnecessary in this function.
	var crList rbacv1.ClusterRoleList
	if err := kt.List(&crList, client.MatchingLabels(kt.labels())); err != nil {
		return err
	}
	for _, cr := range crList.Items {
		if err := kt.DeleteIfExists(&cr); err != nil {
			return err
		}
	}
	return nil
}

// Delete all clusterRoleBindings created by the CR
func (kt *kubeturbo) cleanUpClusterRolebinding() error {
	var crbList rbacv1.ClusterRoleBindingList
	if err := kt.List(&crbList, client.MatchingLabels(kt.labels())); err != nil {
		return err
	}
	for _, crb := range crbList.Items {
		if err := kt.DeleteIfExists(&crb); err != nil {
			return err
		}
	}
	return nil
}
