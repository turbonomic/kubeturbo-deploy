#!/usr/bin/env sh

SCRIPT_DIR="$(cd "$(dirname $0)" && pwd)"
ROOT_DIR="${SCRIPT_DIR}/.."
OPERATOR_YAML_DIR="${ROOT_DIR}/deploy/kubeturbo_operator_yamls"
KUBETURBO_YAML_DIR="${ROOT_DIR}/deploy/kubeturbo_yamls"
TEMP_DIR=$(mktemp -d)

# Some files that assume to be existed before execute the script
CRD_SOURCE="${ROOT_DIR}/config/crd/bases/charts.helm.k8s.io_kubeturbos.yaml"
CR_SOURCE="${ROOT_DIR}/config/samples/charts_v1_kubeturbo.yaml"
CRED_SOURCE="${KUBETURBO_YAML_DIR}/turbo_opsmgr_credentials_secret_sample.yaml"

# Inherited from environment, anticipate to work from the Makefile
NAMESPACE=${NAMESPACE:-"turbo"}
LOCALBIN=${LOCALBIN:-"${ROOT_DIR}/bin"}
KUSTOMIZE=${KUSTOMIZE:-"${LOCALBIN}/kustomize"}

# Function to compose operator full yaml samples using master files
# 
# Listing of all the master files:
# - config/crd/bases/charts.helm.k8s.io_kubeturbos.yaml
# - config/samples/charts_v1_kubeturbo.yaml
# - deploy/kubeturbo_yamls/turbo_opsmgr_credentials_secret_sample.yaml
# 
# Listing of all generated files:
# - deploy/kubeturbo_operator_yamls/kubeturbo_operator_full.yaml
# - deploy/kubeturbo_operator_yamls/kubeturbo_operator_least_admin_full.yaml
# - deploy/kubeturbo_operator_yamls/kubeturbo_operator_reader_full.yaml
# - (deprecating) deploy/kubeturbo_yamls/turbo_kubeturbo_operator_full.yaml
# - (deprecating) deploy/kubeturbo_yamls/turbo_kubeturbo_operator_least_admin_full.yaml
# - (deprecating) deploy/kubeturbo_yamls/turbo_kubeturbo_operator_reader_full.yaml
main() {
    # To ensure all the generated files are up-to-date
    update_operator_bundle

    # Check if prerequisite files exists
    file_check

    # Copy over CRD and Sample CR to the exposing directory
    cp "${CRD_SOURCE}" "${OPERATOR_YAML_DIR}/kubeturbo_crd.yaml"
    cp "${CR_SOURCE}" "${OPERATOR_YAML_DIR}/kubeturbo_sample_cr.yaml"

    # Generate operator full yaml at the temp dir
    operator_yaml_path="${TEMP_DIR}/operator.yaml"
    generate_operator_yaml "${operator_yaml_path}"

    # Concat operator file, CR and turbonomic-credentials to build operator_full yaml files
    echo "Start generate operator full files..."
    cat <<-EOF | sed 's/namespace: .*/namespace: '${NAMESPACE}'/g' > "${OPERATOR_YAML_DIR}/kubeturbo_operator_full.yaml"
	$(cat "${operator_yaml_path}")
	---
	$(cat "${CRED_SOURCE}")
	---
	$(cat "${OPERATOR_YAML_DIR}/kubeturbo_sample_cr.yaml")
	EOF
    sed 's|roleName: cluster-admin|roleName: turbo-cluster-admin|' "${OPERATOR_YAML_DIR}/kubeturbo_operator_full.yaml" > "${OPERATOR_YAML_DIR}/kubeturbo_operator_least_admin_full.yaml"
    sed 's|roleName: cluster-admin|roleName: turbo-cluster-reader|' "${OPERATOR_YAML_DIR}/kubeturbo_operator_full.yaml" > "${OPERATOR_YAML_DIR}/kubeturbo_operator_reader_full.yaml"

    # Keep old files for file migration, will deprecate operator full yaml files under deploy/kubeturbo_yamls folder once done (deprecating)
    cp "${OPERATOR_YAML_DIR}/kubeturbo_operator_full.yaml" "${KUBETURBO_YAML_DIR}/turbo_kubeturbo_operator_full.yaml"
    cp "${OPERATOR_YAML_DIR}/kubeturbo_operator_least_admin_full.yaml" "${KUBETURBO_YAML_DIR}/turbo_kubeturbo_operator_least_admin_full.yaml"
    cp "${OPERATOR_YAML_DIR}/kubeturbo_operator_reader_full.yaml" "${KUBETURBO_YAML_DIR}/turbo_kubeturbo_operator_reader_full.yaml"

    echo "Done"
}

# Function to ensure if all prerequisite files exists
file_check() {
    files="${CRD_SOURCE} ${CR_SOURCE} ${CRED_SOURCE}"
    for f in ${files}; do
        if [ ! -f "${f}" ]; then
            echo "File not found: ${f}"
            exit 1
        fi
    done
}

# Function to ensure the basic yamls (CRD and operator bundle yaml) are synced up
update_operator_bundle() {
    # Move to root dir 
    cd "${ROOT_DIR}" || exit 1

    # Use make command to ensure the bundle and crd file are up-to-date
    make export_operator_yaml_bundle
}

# Function to generate a yaml with operator deployment and RBAC only
generate_operator_yaml() {
    output_file=${1:-"${TEMP_DIR}/operator_full.yaml"}
    temp_operator_full_dir="${ROOT_DIR}/config/operator_full"
    mkdir -p "${temp_operator_full_dir}"

    # Build temporary yaml with only operator deployment and its RBAC
    cat <<-EOF > "${temp_operator_full_dir}/kustomization.yaml"
	---
	apiVersion: kustomize.config.k8s.io/v1beta1
	kind: Kustomization
	namespace: "${NAMESPACE}"
	resources:
	- ../rbac
	- ../manager
	EOF
    ${KUSTOMIZE} build "${temp_operator_full_dir}" > ${output_file}
    echo "Generated the operator full yaml at: ${output_file}"

    # Cleanup
    rm -rf "${temp_operator_full_dir}"
}

main
