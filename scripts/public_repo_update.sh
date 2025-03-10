#!/usr/bin/env bash

set -e

if [ -z "$1" ]; then
    echo "Error: script argument VERSION is not specified."
    exit 1
fi
VERSION=$1

if [ -z "${PUBLIC_GITHUB_TOKEN}" ]; then
    echo "Error: PUBLIC_GITHUB_TOKEN environment variable is not set"
    exit 1
fi
TC_PUBLIC_REPO=turbonomic-container-platform

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SRC_DIR=${SCRIPT_DIR}/../deploy
OUTPUT_DIR=${SCRIPT_DIR}/../_output
HELM=${SCRIPT_DIR}/../bin/helm

if ! command -v ${HELM} > /dev/null 2>&1; then
    HELM=helm
    if ! command -v helm > /dev/null 2>&1; then
        echo "Error: helm could not be found."
        exit 1
    fi
fi

if ! command -v git > /dev/null 2>&1; then
    echo "Error: git could not be found."
    exit 1
fi

echo "===> Cloning public repo..."; 
mkdir ${OUTPUT_DIR}
cd ${OUTPUT_DIR}
git clone https://${PUBLIC_GITHUB_TOKEN}@github.com/IBM/${TC_PUBLIC_REPO}.git
cd ${TC_PUBLIC_REPO}

echo "===> Cleanup existing files"
rm -rf kubeturbo 
mkdir -p kubeturbo/operator
mkdir -p kubeturbo/yamls
mkdir -p kubeturbo/scripts
cd kubeturbo

# copy helm chart
echo "===> Copy helm chart files"
cp -r "${SRC_DIR}/kubeturbo" helm_chart 

# copy operator files
echo "===> Copy Operator files"
cd operator
cp "${SRC_DIR}/kubeturbo_operator_yamls/"*.yaml .

# copy yaml files
echo "===> Copy yaml files"
cd ../yamls
find "${SRC_DIR}/kubeturbo_yamls" -type f \( -name "kubeturbo*.yaml" -o -name "step3*.yaml" -o -name "turbo-reader.yaml" -o -name "turbo-admin.yaml" \) -exec cp {} . \;

# copy script files
echo "===> Copy script files"
cd ../scripts
cp "${SCRIPT_DIR}/../scripts/oauth2_probe_migration.sh" .

# Insert current version
echo "===> Updating Turbo version in yaml files"
cd ..
sed -i.bak "s|version: 1.0.0|version: ${VERSION}|" helm_chart/Chart.yaml
find ./ -type f -name '*.y*' -exec sed -i.bak "s|<Turbo_version>|${VERSION}|g" {} +
find ./ -name '*.bak' -type f -delete
find ./ -name '*.md' -type f -delete
echo "See the [documentation](https://www.ibm.com/docs/en/tarm/latest?topic=targets-connecting-kubernetes-clusters)" > README.md

# commit all modified source files to the public repo
echo "===> Commit modified files to public repo"
cd .. 
git add .
if ! git diff --quiet --cached; then
    git commit -m "kubeturbo deployment ${VERSION}"
    git push
else
    echo "No changed files"
fi

# package the helm chart and upload to helm repo
echo "===> Package helm chart"
${HELM} package kubeturbo/helm_chart -d ${OUTPUT_DIR}

echo "===> Update helm chart index"
git switch gh-pages
cp index.yaml ..
mkdir -p downloads/kubeturbo
cp ${OUTPUT_DIR}/kubeturbo-${VERSION}.* downloads/kubeturbo/ 
${HELM} repo index .. --url https://ibm.github.io/${TC_PUBLIC_REPO}/downloads/kubeturbo --merge index.yaml
cp ../index.yaml . 

# commit packaged helm chart
echo "===> Commit packaged helm chart to helm chart repo"
git add .
git commit -m "kubeturbo helm chart ${VERSION}"
git push

# cleanup
rm -rf ${OUTPUT_DIR}

echo ""
echo "Update public repo complete."
