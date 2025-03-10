#!/bin/bash

# To run with make:
# make helm-test
# To run locally outside of make:
# KUBECONFIG=<ABSOLUTE_PATH_TO_CONFIG_FILE> VERSION=<DESIRED_KUBETURBO_VERSION> scripts/kubeturbo_deployment_helm_test.sh
# If kubectl or helm are not installed within $PATH, then need to further add HELM=<PATH_TO_HELM_EXECUTABLE> KUBECTL=<PATH_TO_KUBECTL_EXECUTABLE> 


check_resources() {
    RESOURCE_TYPE=$1
    RESOURCE_NAME=$2
    echo "Checking if ${RESOURCE_TYPE} is created..."
    if [[ -z $(${KUBECTL} -n ${NAMESPACE} get ${RESOURCE_TYPE} | grep  ${RESOURCE_NAME}) ]]; then
        echo "Error: failed to create ${RESOURCE_TYPE} ${RESOURCE_NAME}"
        exit 1
    fi
}

VERSION="${VERSION:-''}"
KUBECONFIG="${KUBECONFIG:-''}"
KUBECTL="${KUBECTL:-$(command -v kubectl)}"
KUBECTL="${KUBECTL} --kubeconfig=${KUBECONFIG}"  ## Empty KUBECONFIG will use default to use./kube/config
HELM="${HELM:-$(command -v helm)}"
HELM="${HELM} --kubeconfig=${KUBECONFIG}"  ## Empty KUBECONFIG will use default to use./kube/config

HELM_INSTALL_DIR="./deploy/kubeturbo/"

SERVER_VERSION="${VERSION}"
KUBETURBO_VERSION="${SERVER_VERSION}"

ROLE_NAME="turbo-cluster-admin"
RB_NAME="turbo-all-binding-helm-test"
SA_NAME="turbo-user-helm-test"

NAMESPACE="turbonomic-helm-test"

TARGET_NAME="Kind-helm-test"
TURBO_SERVER_URL="https:\/\/dummy-server"


echo "Using kubeturbo version: ${KUBETURBO_VERSION}"
echo "Using kubeconfig file: ${KUBECONFIG}"

#construct and save to new values.yaml
sed -e "s/tag:.*/tag: $KUBETURBO_VERSION/g" \
    -e "s/roleName:.*/roleName: $ROLE_NAME/g" \
    -e "s/roleBinding:.*/roleBinding: $RB_NAME/g" \
    -e "s/serviceAccountName:.*/serviceAccountName: $SA_NAME/g" \
    -e "s/version:.*/version: $SERVER_VERSION/g" \
    -e "s/turboServer:.*/turboServer: $TURBO_SERVER_URL/g" \
    -e "s/targetName:.*/targetName: $TARGET_NAME/g" \
    -e "s/opsManagerUserName:.*//g" \
    -e "s/opsManagerPassword:.*//g" \
    "${HELM_INSTALL_DIR}/values.yaml" > "${HELM_INSTALL_DIR}/values-$TARGET_NAME.yaml"	

# create test namespace
${KUBECTL} create ns ${NAMESPACE}

# install chart
RELEASE_NAME=kubeturbo-helm-test
echo "Uninstalling release if it exists already"
${HELM} uninstall ${RELEASE_NAME} -n ${NAMESPACE}

echo "Installing kubeturbo through helm"
${HELM} install ${RELEASE_NAME} ${HELM_INSTALL_DIR} --values ${HELM_INSTALL_DIR}/values-$TARGET_NAME.yaml -n ${NAMESPACE}

COUNTER=1
while [[ -z $(${KUBECTL} -n ${NAMESPACE} get pod | grep ${RELEASE_NAME} | grep Running) ]]
do 
    if [ $COUNTER -eq 10 ]; then
        echo "Time out waiting for pod to start"
        echo "----------------------------------------------------------------------------------------"
        echo "Generated values.yaml:"
        cat ${HELM_INSTALL_DIR}/values-$TARGET_NAME.yaml
        echo "----------------------------------------------------------------------------------------"
        echo "Pod status and events:"
        ${KUBECTL} -n ${NAMESPACE} get pod | grep ${RELEASE_NAME}
        ${KUBECTL} -n ${NAMESPACE} get events --sort-by='.lastTimestamp' | grep ${RELEASE_NAME}
        exit 1
    fi
    echo "Waiting for kubeturbo pod to start"
    sleep 5
    COUNTER=$[$COUNTER +1]
done

check_resources serviceaccount ${SA_NAME}
check_resources clusterrole ${ROLE_NAME}
check_resources clusterrolebinding ${RB_NAME}
check_resources configmap turbo-config-${RELEASE_NAME}
echo "Test passed!"

echo "Uninstalling kubeturbo"
${HELM} uninstall ${RELEASE_NAME} -n ${NAMESPACE}

echo "Deleting test values.yaml"
rm -f ${HELM_INSTALL_DIR}/values-$TARGET_NAME.yaml

echo "Deleting test namespace"
${KUBECTL} delete ns ${NAMESPACE}
