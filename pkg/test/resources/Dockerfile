FROM docker:20.10.14-alpine3.15

ARG OSM_VERSION
ARG GADGET_VERSION

# Add binaries/archives
RUN apk add gettext && \
    wget -O /usr/local/bin/kind https://kind.sigs.k8s.io/dl/v0.12.0/kind-linux-amd64 && \
    wget -O /helm.tar.gz https://get.helm.sh/helm-v3.8.2-linux-amd64.tar.gz && \
    wget -O /usr/local/bin/kubectl https://dl.k8s.io/release/v1.23.5/bin/linux/amd64/kubectl && \
    wget -O /osm.tar.gz https://github.com/openservicemesh/osm/releases/download/v$OSM_VERSION/osm-v$OSM_VERSION-linux-amd64.tar.gz && \
    wget -O /kubectl-gadget.tar.gz https://github.com/inspektor-gadget/inspektor-gadget/releases/download/v$GADGET_VERSION/kubectl-gadget-linux-amd64-v$GADGET_VERSION.tar.gz

# Set file modes and extract
RUN chmod 755 /usr/local/bin/kind && \
    chmod 755 /usr/local/bin/kubectl && \
    tar -zxvf /helm.tar.gz && mv /linux-amd64/helm /usr/local/bin/helm && \
    tar -zxvf /osm.tar.gz && mv /linux-amd64/osm /usr/local/bin/osm && \
    tar -zxvf /kubectl-gadget.tar.gz && mv /kubectl-gadget /usr/local/bin/kubectl-gadget

# Copy resources
ADD tools-resources /resources
ADD deployment /deployment
