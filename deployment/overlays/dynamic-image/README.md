# Dynamic Image Overlay Template

This overlay is used to deploy specific Periscope Docker images to a cluster.

(It is really a *template* for an overlay, rather than an actual overlay, because although `Kustomize` supports dynamic configuration for `ConfigMap` and `Secret` resources via `.env` files, it does not allow dynamically specifying image names/tags.)

The typical use-case would be to manually test an in-development build of Periscope on a managed cluster. You might do this if your development environment does not have Docker/Kind available, or does not support the feature you wish to test. One example is running Periscope on Windows nodes (further notes on creating a cluster with Windows nodes is [below](#creating-a-windows-cluster)).

If the resources are being deployed to a managed cluster, your Periscope build will need to be published to an external image registry. The options described here are:

- ACR: If you have local (not yet pushed) changes that you want to test on a managed AKS cluster, you can publish images to an ACR from your local filesystem.
- GHCR: If you have pushed changes to a fork of the Periscope repository on which you have permission to publish releases, you can publish images to GHCR from a branch in your repository.

## Publishing Images

The steps required to build and publish an image depend on the registry you're publishing to.

### Publishing to ACR

#### 1. Initial Setup

If you're not a subscription owner, you can configure the Periscope deployment to authenticate using the ACR's Admin account. To enable the admin account on the ACR, run:

```sh
acr_name=...
az acr update -n $acr_name --admin-enabled true

```

#### 2. Set Environment Variables for ACR

These variables are needed for both publishing the images and deploying Periscope. The image tag can be set to whatever you like.

```sh
acr_name=...
export IMAGE_TAG=...
export IMAGE_NAME=${acr_name}.azurecr.io/aks/periscope
```

#### 3. Run an ACR Build

You can build and publish both Linux and Windows images using the `az acr build` command:

```sh
# Build images for each required platform.
az acr build --registry $acr_name -f ./builder/Dockerfile.linux -t $IMAGE_NAME:$IMAGE_TAG-linux --platform linux/amd64 .
az acr build --registry $acr_name -f ./builder/Dockerfile.windows -t $IMAGE_NAME:$IMAGE_TAG-win2019 --build-arg BASE_IMAGE=mcr.microsoft.com/windows/nanoserver:ltsc2019 --platform windows/amd64 .
az acr build --registry $acr_name -f ./builder/Dockerfile.windows -t $IMAGE_NAME:$IMAGE_TAG-win2022 --build-arg BASE_IMAGE=mcr.microsoft.com/windows/nanoserver:ltsc2022 --platform windows/amd64 .

# Create a cross-platform manifest file
az acr login -n $acr_name
docker manifest create $IMAGE_NAME:$IMAGE_TAG $IMAGE_NAME:$IMAGE_TAG-linux $IMAGE_NAME:$IMAGE_TAG-win2019 $IMAGE_NAME:$IMAGE_TAG-win2022
docker manifest push $IMAGE_NAME:$IMAGE_TAG
```

#### 4. Authorise Cluster to use ACR

Any cluster can be used, we use Azure Kubernetes Cluster (AKS). If the AKS cluster and ACR are in a subscription in which you have the `Owner` role, you can attach the ACR to your cluster without the need to supply credentials in the deployment spec (the `Owner` role is needed for ACR role assignments).

```sh
rg=...
aks_name=...
acr_name=...
az aks update --resource-group $rg --name $aks_name --attach-acr $acr_name
```

### Publishing to GHCR

#### 1. Run the Publish Workflow

Make a note of the latest version heading in [the changelog](../../../CHANGELOG.md). This will be used for the published image tags.

Run the [Building and Pushing to GHCR](../../../.github/workflows/build-and-publish.yml) workflow in GitHub Actions (making sure to select the correct branch).

#### 2. Set Environment Variables for GHCR

These variables will be needed for deploying Periscope. Fill in the name of the GitHub fork, and the published image tag.

```sh
repo_username=...
export IMAGE_TAG=...
export IMAGE_NAME=ghcr.io/${repo_username}/aks/periscope
```

#### 3. Ensure Packages are Public

This only needs to be done once for each of the Linux and Windows packages. Under Package Settings in GitHub, set each package's visibility to 'public'.

### Local image registry

You can build and run Periscope locally in a local `Kind` cluster and local image registry such as `docker`. Because `Kind` runs on Linux only, the Linux `DaemonSet` will refer to the locally-built image, whereas the Windows `DaemonSet` will refer to the latest published production Windows MCR image (to test Windows changes, use previous options).

#### 1. Build Locally (Linux only)

Build and load the image in `Kind`. If it's not, the pod will fail trying to pull the image (because it's local).

```sh
docker build -f ./builder/Dockerfile.linux -t periscope-local .
# Include a --name argument here if not using the default kind cluster.
kind load docker-image periscope-local
```

## Setting up Configuration Data

To run correctly, Periscope requires some storage account configuration that is different for each user. It also has some optional 'diagnostic' configuration (node log locations, etc.). The environment files are `gitignore`d to avoid committing any credentials or user-specific configuration to source control.

Periscope loads `Secret` from file `.env.secret` to acquire access to the storage account.

```sh
# Create a SAS
sub_id=...
stg_account=...
blob_container=...
sas_expiry=`date -u -d "30 minutes" '+%Y-%m-%dT%H:%MZ'`
sas=$(az storage account generate-sas \
    --account-name $stg_account \
    --subscription $sub_id \
    --permissions rlacw \
    --services b \
    --resource-types sco \
    --expiry $sas_expiry \
    -o tsv)

# Create a clean overlay folder
rm -rf ./deployment/overlays/temp && mkdir ./deployment/overlays/temp

# Set up storage configuration data for Kustomize
cat <<EOF > ./deployment/overlays/temp/.env.secret
AZURE_BLOB_ACCOUNT_NAME=${stg_account}
AZURE_BLOB_SAS_KEY=?${sas}
AZURE_BLOB_CONTAINER_NAME=${blob_container}
EOF
```

You can also override diagnostic configuration variables:

```sh
echo "DIAGNOSTIC_KUBEOBJECTS_LIST=kube-system default" > ./deployment/overlays/temp/.env.config
```

If using ACR's admin account credentials to access the Periscope image

```sh
acr_name=...
acr_username=$(az acr credential show -n $acr_name --query username --output tsv)
acr_password=$(az acr credential show -n $acr_name --query "passwords[0].value" --output tsv)
cat <<EOF > ./deployment/overlays/temp/acr.dockerconfigjson
{
    "auths": {
        "${acr_name}.azurecr.io": {
            "username": "${acr_username}",
            "password": "${acr_password}"
        }
    }
}
EOF
```

## Deploying Periscope

Make sure all necessary config files are in-place:

- `.env.secret`
- `.env.config` 
- `acr.dockerconfigjson`

otherwise create the files if they don't already exist

```sh
# Create the required config files if they don't already exist
touch ./deployment/overlays/temp/.env.config 
touch ./deployment/overlays/temp/acr.dockerconfigjson
```

### Local Clusters

This approach assumes that you have built an image locally and loaded to `Kind` as in [Local Development](#local-image-registry). It will deploy to its own namespace, `aks-periscope-dev` to avoid conflicts with any existing Periscope deployment.

Once the `.env` files are in place, `Kustomize` has all the information it needs to generate the `yaml` resource specification for Periscope. We need to make sure this doesn't get into source control, so it is stored in `gitignore`d `.env` files.

```sh
# Ensure kubectl has the right cluster context
export KUBECONFIG=...
# Deploy
kubectl apply -k ./deployment/overlays/dev
```

### All clusters

First ensure your environment variables are set up. See notes for [ACR](#2-set-environment-variables-for-acr) and [GHCR](#2-set-environment-variables-for-ghcr).

Next you can use `envsubst` to generate a `Kustomize` overlay from the template (this is placed in the `overlays/temp` directory, which is excluded from source control), and deploy it with `kubectl`.

```sh
# Generate the kustomization.yaml
cat ./deployment/overlays/dynamic-image/kustomization.template.yaml | envsubst > ./deployment/overlays/temp/kustomization.yaml

# Ensure kubectl has the right cluster context
export KUBECONFIG=...

# Deploy
kubectl apply -k ./deployment/overlays/temp
```

Each time we want Periscope to run, we supply a new run ID for it. This can be done with:

```sh
run_id=$(date -u '+%Y-%m-%dT%H-%M-%SZ')
kubectl patch configmap -n aks-periscope diagnostic-config -p="{\"data\":{\"DIAGNOSTIC_RUN_ID\": \"$run_id\"}}"
```

### Deploy Periscope to AKS with ACR

A [utlity script](./deploy_dev_linux.sh) is provided to build image and deploy to the AKS cluster in one line:

```sh
./deploy_dev_linux.sh "<SUB_ID>" "<rg>" "<cluster-name>" "<acr-name>" "<storage-account>" "<blob-container-name" "<image-tag>" "<docker/acr>"
```

## Footnotes

## Using other clusters

### Creating a Windows Cluster

This section contains notes on creating a Windows cluster in AKS. It's documented here because creating a cluster with Windows nodes currently takes a little bit of command-line work.

```sh
# Variables for subscription ID, resource group, cluster name and node-pool name
# node pool "may only contain lowercase alphanumeric characters and must begin with a lowercase letter"
sub_id=...
rg=...
aks_name=...
nodepool_name=...
# Create the cluster with a system nodepool (Linux)
az aks create \
    --subscription $sub_id \
    --resource-group $rg \
    --name $aks_name \
    --node-count 2 \
    --enable-addons monitoring \
    --generate-ssh-keys \
    --windows-admin-username WindowsUser1 \
    --vm-set-type VirtualMachineScaleSets \
    --network-plugin azure
# Create an additional user nodepool (Windows)
az aks nodepool add \
    --subscription $sub_id \
    --resource-group $rg \
    --cluster-name $aks_name \
    --os-type Windows \
    --name $nodepool_name \
    --node-count 1
# Set the kubectl context to the new cluster
az aks get-credentials \
    --subscription $sub_id \
    --resource-group $rg \
    --name $aks_name
```
