#!/bin/bash

# To run with make:
# make yaml-test
# To run locally outside of make:
# KUBECONFIG=<ABSOLUTE_PATH_TO_CONFIG_FILE> VERSION=<DESIRED_KUBETURBO_VERSION> scripts/kubeturbo_deployment_helm_test.sh
# If kubectl is not installed within $PATH, then need to further add KUBECTL=<PATH_TO_KUBECTL_EXECUTABLE> 


check_resources() {
    RESOURCE_TYPE=$1
    RESOURCE_NAME=$2
    echo "Checking if ${RESOURCE_TYPE} is created..."
    if [[ -z $(${KUBECTL} -n ${NAMESPACE} get ${RESOURCE_TYPE} | grep  ${RESOURCE_NAME}) ]]; then
        echo "Error: failed to create ${RESOURCE_TYPE} ${RESOURCE_NAME}"
        SUCCESS_FLAG=0
    fi
}

run_yaml_test() {
    YAML_FILE=$1
    ROLE_NAME=$2
    NAMESPACE="yaml-test-${ROLE_NAME}"

    echo "----------------------------------------------------------------------------------------"
    echo "Starting test for ${YAML_FILE}"

    # remove ns (by splitting after first occurrence of "name: turbo") then substitute with desired values
    sed -e '1,/^  name\: turbo$/d' \
        -e "s/<Turbo_version>/${KUBETURBO_VERSION}/g" \
        -e "s/\"turboServer\": \"<https:\/\/Turbo_Server_URL_or_IP_address>\"/\"turboServer\": \"$TURBO_SERVER_URL\"/g" \
        -e "s/username: BASE64encodedValue/username: ${SAMPLE_USERNAME}/g" \
        -e "s/password: BASE64encodedValue/password: ${SAMPLE_PASSWORD}/g" \
        -e "s/namespace: turbo/namespace: ${NAMESPACE}/g" \
        ${YAML_DIR}/${YAML_FILE} > ${YAML_DIR}/test-${YAML_FILE}
    
    # Create test ns
    ${KUBECTL} create ns ${NAMESPACE}
    # Apply yaml to install kubeturbo
    ${KUBECTL} apply -f ${YAML_DIR}/test-${YAML_FILE}

    COUNTER=1
    while [[ -z $(${KUBECTL} -n ${NAMESPACE} get pod | grep kubeturbo | grep Running) ]]
    do 
        if [ $COUNTER -eq 10 ]; then
            echo "Time out waiting for pod to start"
            echo "----------------------------------------------------------------------------------------"
            echo "Generated yaml file:"
            cat ${YAML_DIR}/test-${YAML_FILE}
            echo "----------------------------------------------------------------------------------------"
            echo "Pod status and events:"
            ${KUBECTL} -n ${NAMESPACE} get pod | grep kubeturbo
            ${KUBECTL} -n ${NAMESPACE} get events --sort-by='.lastTimestamp' | grep "kubeturbo"
            echo "Test failed!"
            SUCCESS_FLAG=0
            return
        fi
        echo "Waiting for kubeturbo pod to start"
        sleep 5
        COUNTER=$[$COUNTER +1]
    done

    check_resources serviceaccount ${SA_NAME}
    check_resources clusterrole ${ROLE_NAME}
    check_resources clusterrolebinding ${RB_NAME}
    check_resources configmap ${CONFIGMAP_NAME}
    echo "Test passed!"

    echo "Uninstalling kubeturbo"
    ${KUBECTL} delete -f ${YAML_DIR}/test-${YAML_FILE}

    echo "Deleting test values.yaml"
    rm -f ${YAML_DIR}/test-${YAML_FILE}

     echo "Deleting namespace ${NAMESPACE}"
    ${KUBECTL} delete ns ${NAMESPACE}
}

VERSION="${VERSION:-''}"
KUBECONFIG="${KUBECONFIG:-''}"
KUBECTL="${KUBECTL:-$(command -v kubectl)}"
KUBECTL="${KUBECTL} --kubeconfig=${KUBECONFIG}"  ## Empty KUBECONFIG will use default to use./kube/config

KUBETURBO_VERSION="${VERSION}"

SA_NAME="turbo-user"
RB_NAME="turbo-all-binding-kubeturbo-turbo"
CONFIGMAP_NAME="turbo-config"

TURBO_SERVER_URL="https:\/\/dummy-server"
SAMPLE_USERNAME=$(echo "user1" | base64)
SAMPLE_PASSWORD=$(echo "password1" | base64)

YAML_DIR="deploy/kubeturbo_yamls"
LEAST_ADMIN_FILE="kubeturbo_least_admin_full.yaml"
LEAST_ADMIN_ROLE_NAME="turbo-cluster-admin"
READER_FILE="kubeturbo_reader_full.yaml"
READER_ROLE_NAME="turbo-cluster-reader"
CLUSTER_ADMIN_FILE="kubeturbo_full.yaml"
CLUSTER_ADMIN_ROLE_NAME="cluster-admin"

SUCCESS_FLAG=1

echo "Using kubeturbo version: ${KUBETURBO_VERSION}"
echo "Using kubeconfig file: ${KUBECONFIG}"

run_yaml_test ${LEAST_ADMIN_FILE} ${LEAST_ADMIN_ROLE_NAME}
run_yaml_test ${READER_FILE} ${READER_ROLE_NAME}
run_yaml_test ${CLUSTER_ADMIN_FILE} ${CLUSTER_ADMIN_ROLE_NAME}

if [ ${SUCCESS_FLAG} -eq 1 ]; then
    echo "Summmary: All tests passed!"
    exit 0
else
    echo "Summary: One or more tests failed!"
    exit 1
fi