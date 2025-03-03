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

package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	ktv1 "github.ibm.com/turbonomic/kubeturbo-deploy/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2" //nolint:golint,revive
)

type K8SAssets struct {
	ServiceAccount     string
	ClusterRole        string
	ClusterRoleBinding string
	Deployment         string
	ConfigMap          string
}

const (
	prometheusOperatorVersion = "v0.68.0"
	prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
		"releases/download/%s/bundle.yaml"

	certmanagerVersion = "v1.5.3"
	certmanagerURLTmpl = "https://github.com/jetstack/cert-manager/releases/download/%s/cert-manager.yaml"
)

var (
	KUBECTL             = EnvLookUp("KUBECTL", "kubectl")
	KIND                = EnvLookUp("KIND", "kind")
	KIND_CLUSTER        = EnvLookUp("KIND_CLUSTER", "kind")
	KIND_KUBECONFIG     = EnvLookUp("KIND_KUBECONFIG", "~/.kube/config")
	NAMESPACE           = EnvLookUp("NAMESPACE", "turbonomic")
	REGISTRY            = EnvLookUp("REGISTRY", "e2e-test")
	OPERATOR_NAME       = EnvLookUp("OPERATOR_NAME", "kubeturbo-operator")
	VERSION             = EnvLookUp("VERSION", "8.13.6")
	KIND_KUBECONFIG_STR = fmt.Sprintf("--kubeconfig=%v", KIND_KUBECONFIG)
	LOGGING_LEVEL       = EnvLookUp("TESTING_LOGGING_LEVEL", "INFO")
	DEFAULT_KT_VERSION  = EnvLookUp("DEFAULT_KUBETURBO_VERSION", VERSION)
	OPERATOR_ASSETS     = K8SAssets{
		ServiceAccount:     "serviceaccount/kubeturbo-operator",
		ClusterRole:        "clusterrole.rbac.authorization.k8s.io/kubeturbo-operator",
		ClusterRoleBinding: "clusterrolebinding.rbac.authorization.k8s.io/kubeturbo-operator",
		Deployment:         "deployment.apps/kubeturbo-operator",
	}
	LOG_LEVELS = map[string]int{
		"ERROR": 1,
		"WARN":  2,
		"INFO":  3,
		"DEBUG": 4,
	}
)

func LogMessage(level string, format string, args ...interface{}) {
	normalizedLogLevel := strings.ToUpper(LOGGING_LEVEL)
	currentLogLevel, ok := LOG_LEVELS[normalizedLogLevel]
	if !ok {
		LOGGING_LEVEL = "INFO"
		LogMessage("WARN", "The given log level %s is invalid, default to INFO level", normalizedLogLevel)
		return
	}

	normalizedLogLevel = strings.ToUpper(level)
	messageLogLevel, ok := LOG_LEVELS[normalizedLogLevel]
	if !ok {
		LogMessage("WARN", "The message sets to an invalid log level %s default to INFO level", level)
		LogMessage("INFO", format, args...)
		return
	}

	if messageLogLevel <= currentLogLevel {
		fmt.Fprintf(GinkgoWriter, "[%s] %s\n", normalizedLogLevel, fmt.Sprintf(format, args...))
	}
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
func InstallPrometheusOperator() error {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command(KUBECTL, "create", "-f", url)
	_, err := Run(cmd)
	return err
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) ([]byte, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		fmt.Fprintf(GinkgoWriter, "chdir dir: %s\n", err)
	}

	cmd_name := cmd.Args[0]
	if cmd_name == KUBECTL {
		cmd.Args = append(cmd.Args, KIND_KUBECONFIG_STR)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	LogMessage("INFO", "> %s", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed with error: (%v) %s", command, err, string(output))
	}
	LogMessage("DEBUG", "%s", output)
	return output, nil
}

// UninstallPrometheusOperator uninstalls the prometheus
func UninstallPrometheusOperator() {
	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
	cmd := exec.Command(KUBECTL, "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		LogMessage("WARN", "%s", err.Error())
	}
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command(KUBECTL, "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		LogMessage("WARN", "%s", err.Error())
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command(KUBECTL, "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	cmd = exec.Command(KUBECTL, "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// LoadImageToKindCluster loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	kindOptions := []string{"load", "docker-image", name, "--name", KIND_CLUSTER}
	cmd := exec.Command(KIND, kindOptions...)
	_, err := Run(cmd)
	return err
}

func SwitchToContext(context string) (err error) {
	LogMessage("INFO", "Switching to k8s context: %s", context)
	cmd := exec.Command(KUBECTL, "config", "use-context", context)
	_, err = Run(cmd)
	return
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, err
	}
	wd = strings.Replace(wd, "/test/e2e", "", -1)
	return wd, nil
}

func EnvLookUp(env_var string, default_val string) (val string) {
	val = default_val
	if v, ok := os.LookupEnv(env_var); ok {
		val = v
	}
	fmt.Fprintf(GinkgoWriter, "Addressing %s as: %s\n", env_var, val)
	os.Setenv(env_var, val)
	return
}

func CurrentK8sContext() (currentContext string) {
	cmd := exec.Command(KUBECTL, "config", "current-context")
	output, _ := Run(cmd)
	currentContext = GetNonEmptyLines(string(output))[0]
	LogMessage("INFO", "Current k8s context is: %s", currentContext)
	return
}

func VerifyK8sAssets(assets K8SAssets) error {
	var cmd *exec.Cmd
	var err error
	var errs []error

	// To verify if targeting k8s objects are created
	k8s_targets := []string{assets.ServiceAccount, assets.ClusterRoleBinding, assets.ClusterRole, assets.Deployment, assets.ConfigMap}
	for _, k8s_target := range k8s_targets {
		if k8s_target == "" {
			continue
		}
		cmd = exec.Command(KUBECTL, "-n", NAMESPACE, "get", k8s_target)
		_, err = Run(cmd)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return SummarizeErrors(errs)
}

func VerifyCRB(assets K8SAssets) error {
	var cmd *exec.Cmd
	var err error
	var errs []error

	// To verify the clusterrole_binding binds to right object
	jsonPath := "-o=jsonpath='{.roleRef.kind}.{.roleRef.apiGroup}/{.roleRef.name}'"
	cmd = exec.Command(KUBECTL, "get", assets.ClusterRoleBinding, jsonPath)
	result, err := Run(cmd)
	if err != nil {
		errs = append(errs, err)
	}

	resultStr := strings.ToLower(strings.Trim(string(result), "'"))
	if resultStr != assets.ClusterRole {
		errs = append(errs, fmt.Errorf("expecting %s binds to %s but got %s", assets.ClusterRoleBinding, assets.ClusterRole, resultStr))
	}

	jsonPath = fmt.Sprintf("-o=jsonpath='{range .subjects[?(@.namespace == \"%s\")]}{.kind}/{.name}{end}'", NAMESPACE)
	cmd = exec.Command(KUBECTL, "get", assets.ClusterRoleBinding, jsonPath)
	result, err = Run(cmd)
	if err != nil {
		errs = append(errs, err)
	}

	resultStr = strings.ToLower(strings.Trim(string(result), "'"))
	if resultStr != assets.ServiceAccount {
		errs = append(errs, fmt.Errorf("expecting %s binds to %s but got %s", assets.ClusterRoleBinding, assets.ServiceAccount, resultStr))
	}

	return SummarizeErrors(errs)
}

func VerifyDeployment(assets K8SAssets) error {
	var cmd *exec.Cmd
	var err error
	var errs []error

	// To verify the deployment uses right serviceaccount
	jsonPath := "-o=jsonpath='serviceaccount/{.spec.template.spec.serviceAccountName}'"
	cmd = exec.Command(KUBECTL, "-n", NAMESPACE, "get", assets.Deployment, jsonPath)
	result, err := Run(cmd)
	if err != nil {
		errs = append(errs, err)
	}

	resultStr := strings.ToLower(strings.Trim(string(result), "'"))
	if resultStr != assets.ServiceAccount {
		errs = append(errs, fmt.Errorf("expecting %s uses %s but got %s", assets.Deployment, assets.ServiceAccount, resultStr))
	}

	// To verify the operator pod runs without any issue
	LogMessage("INFO", "Wait until the deployment %s become ready", assets.Deployment)
	cmd = exec.Command(KUBECTL, "-n", NAMESPACE, "rollout", "status", assets.Deployment, "--timeout=1m")
	_, err = Run(cmd)
	if err != nil {
		errs = append(errs, err)
	}

	return SummarizeErrors(errs)
}

func VerifyImageTag(assets K8SAssets, containerName string, desireTag string) error {
	var cmd *exec.Cmd
	var err error

	jsonPath := fmt.Sprintf("-o=jsonpath='{.spec.template.spec.containers[?(@.name == \"%s\")].image}'", containerName)
	cmd = exec.Command(KUBECTL, "-n", NAMESPACE, "get", assets.Deployment, jsonPath)
	result, err := Run(cmd)
	if err != nil {
		return err
	}

	resultStr := strings.Trim(string(result), "'")
	parts := strings.Split(resultStr, ":")
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}

	if desireTag != tag {
		return fmt.Errorf("expecting %s uses image tag %s but got %s", assets.Deployment, desireTag, tag)
	}

	return nil
}

func SummarizeErrors(errs []error) error {
	var sb strings.Builder

	// Iterate through the errors and append to the builder if not nil.
	for _, err := range errs {
		if err != nil {
			sb.WriteString(err.Error() + "\n")
		}
	}

	if len(sb.String()) == 0 {
		return nil
	}
	return errors.New(sb.String())
}

func WriteToFile(targetFile *os.File, value any) (err error) {
	file, err := os.Create(targetFile.Name())
	if err != nil {
		LogMessage("ERROR", "Failed to open file: %s", err)
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err = encoder.Encode(value); err != nil {
		LogMessage("ERROR", "Fail to encode %s to %s", value, file.Name())
		return err
	}
	return nil
}

func GenerateKtV1CRWithSpec(name string, spec *ktv1.KubeturboSpec) ktv1.Kubeturbo {
	return ktv1.Kubeturbo{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kubeturbo",
			APIVersion: "charts.helm.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: NAMESPACE,
		},
		Spec: *spec,
	}
}

func GenerateKubeturboAssets(kt ktv1.Kubeturbo) K8SAssets {
	// follow the same rule sets in internal/api/kubeturbo/reconciler.go
	serviceAccount := "turbo-user"
	if kt.Spec.ServiceAccountName != "" {
		serviceAccount = kt.Spec.ServiceAccountName
	}

	// follow the same rule sets in internal/api/kubeturbo/reconciler.go
	clusterRole := "cluster-admin"
	if kt.Spec.RoleName != "" {
		clusterRole = kt.Spec.RoleName
		if clusterRole == ktv1.RoleTypeAdmin || clusterRole == ktv1.RoleTypeReadOnly {
			clusterRole = clusterRole + "-" + kt.ObjectMeta.Name + "-" + kt.ObjectMeta.Namespace
		}
	}

	// follow the same rule sets in internal/api/kubeturbo/reconciler.go
	clusterRoleBinding := "turbo-all-binding"
	if kt.Spec.RoleBinding != "" {
		clusterRoleBinding = kt.Spec.RoleBinding
	}
	clusterRoleBinding = clusterRoleBinding + "-" + kt.ObjectMeta.Name + "-" + kt.ObjectMeta.Namespace

	// follow the same rule sets in internal/api/kubeturbo/reconciler.go
	configMap := "turbo-config-" + kt.ObjectMeta.Name

	return K8SAssets{
		Deployment:         fmt.Sprintf("deployment.apps/%s", kt.Name),
		ConfigMap:          fmt.Sprintf("configmap/%s", configMap),
		ServiceAccount:     fmt.Sprintf("serviceaccount/%s", serviceAccount),
		ClusterRoleBinding: fmt.Sprintf("clusterrolebinding.rbac.authorization.k8s.io/%s", clusterRoleBinding),
		ClusterRole:        fmt.Sprintf("clusterrole.rbac.authorization.k8s.io/%s", clusterRole),
	}
}

func GetRunningPodByDeployName(deployName string) (string, error) {
	cmd := exec.Command(KUBECTL,
		"-n", NAMESPACE,
		"get", "pods",
		"-o", "name",
		"--field-selector=status.phase=Running",
		"-l", fmt.Sprintf("app.kubernetes.io/name=%s", deployName),
	)
	result, err := Run(cmd)
	if err != nil {
		return "", err
	}

	resultStr := strings.Trim(string(result), "'")
	LogMessage("INFO", "The current running pod for %s is %s", deployName, resultStr)
	return resultStr, nil
}
