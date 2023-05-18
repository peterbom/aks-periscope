#!/bin/bash

# set up environment to test local changes
set -euo pipefail
echo "$@"

export SUB_ID="${1}"
export RESOURCE_GROUP="${2}"
export AKS_CLUSTER_NAME="${3}"
export ACR="${4}"
export STORAGE_ACCOUNT="${5}"
export BLOB_CONTAINER="${6}"
export IMAGE_TAG="${7}"
export BUILDER="${8}"

# the script needs to run at the root directory
curr_dir=${PWD}
work_dir=$curr_dir

function cleanup() {
    # rm -rf ./deployment/overlays/temp
    cd $curr_dir
}

trap cleanup EXIT


while [ ! -d "${work_dir}/builder" ]
do 
  work_dir="$(dirname "$work_dir")"
done


cd ${work_dir}
echo "set working directory to "${PWD}

az account set --subscription $SUB_ID
echo "login to ACR"

az acr login -n ${ACR}

# build testing images in ACR
echo "building images to deploy"
export IMAGE_NAME=${ACR}.azurecr.io/aks/periscope

if [ $BUILDER = "docker" ];
then
  echo "build images using docker"
  export IMAGE_TAG=${IMAGE_TAG}-linux
  docker build -f ./builder/Dockerfile.linux -t $IMAGE_NAME:$IMAGE_TAG --platform linux/amd64 .
  docker push $IMAGE_NAME:$IMAGE_TAG
else
  echo "build images using az acr"
  az acr build --registry ${ACR} -f ./builder/Dockerfile.linux -t $IMAGE_NAME:$IMAGE_TAG-linux --platform linux/amd64 .
  az acr build --registry ${ACR} -f ./builder/Dockerfile.windows -t $IMAGE_NAME:$IMAGE_TAG-win2019 --build-arg BASE_IMAGE=mcr.microsoft.com/windows/nanoserver:ltsc2019 --platform windows/amd64 .
  az acr build --registry ${ACR} -f ./builder/Dockerfile.windows -t $IMAGE_NAME:$IMAGE_TAG-win2022 --build-arg BASE_IMAGE=mcr.microsoft.com/windows/nanoserver:ltsc2022 --platform windows/amd64 .

  docker manifest create $IMAGE_NAME:$IMAGE_TAG $IMAGE_NAME:$IMAGE_TAG-linux $IMAGE_NAME:$IMAGE_TAG-win2019 $IMAGE_NAME:$IMAGE_TAG-win2022
  docker manifest push $IMAGE_NAME:$IMAGE_TAG
fi



# export env secret
sas_expiry=`date -u -d "30 minutes" '+%Y-%m-%dT%H:%MZ'`
sas=$(az storage account generate-sas \
--account-name $STORAGE_ACCOUNT \
--subscription $SUB_ID \
--permissions rlacw \
--services b \
--resource-types sco \
--expiry $sas_expiry \
-o tsv)

# setup AKS to use ACR 
echo "setup ACR to for AKS"
az aks update --resource-group $RESOURCE_GROUP --name $AKS_CLUSTER_NAME --attach-acr ${ACR}

echo "prepare kustomization to deploy"
rm -rf ./deployment/overlays/temp && mkdir ./deployment/overlays/temp

echo "writing .env.secret file"
cat << EOF > ./deployment/overlays/temp/.env.secret
AZURE_BLOB_ACCOUNT_NAME=${STORAGE_ACCOUNT}
AZURE_BLOB_SAS_KEY=?${sas}
AZURE_BLOB_CONTAINER_NAME=${BLOB_CONTAINER}
EOF

echo "writing acr.dockerconfigjson file"
acr_username=$(az acr credential show -n ${ACR} --query username --output tsv)
acr_password=$(az acr credential show -n ${ACR} --query "passwords[0].value" --output tsv)

cat << EOF > ./deployment/overlays/temp/acr.dockerconfigjson
{
    "auths": {
        "${ACR}.azurecr.io": {
            "username": "${acr_username}",
            "password": "${acr_password}"
        }
    }
}
EOF

echo "use default .env.config file"
touch  ./deployment/overlays/temp/.env.config

# Generate the kustomization.yaml
echo "Generating overlay kustomization.yaml"
cat ./deployment/overlays/dynamic-image/kustomization.template.yaml | envsubst > ./deployment/overlays/temp/kustomization.yaml

echo "deploying artifacts"
kubectl apply -k ./deployment/overlays/temp