package utils

import (
	"fmt"
	"os"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultKubeturboVersionEnvVar = "DEFAULT_KUBETURBO_VERSION"
)

var (
	staticLogger = ctrl.Log.WithName("Operator-static")
)

func GetDefaultKubeturboVersion() (string, error) {
	// Env var DEFAULT_KUBETURBO_VERSION specifies the default Kubeturbo version that
	// the operator should use when the client doesn't specify the version
	version, found := os.LookupEnv(DefaultKubeturboVersionEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", DefaultKubeturboVersionEnvVar)
	}
	staticLogger.Info(fmt.Sprintf("DEFAULT_KUBETURBO_VERSION=%s", version))
	return version, nil
}

// StringInSlice checks if a string is in a slice of strings
func StringInSlice(str string, list []string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}
	return false
}
