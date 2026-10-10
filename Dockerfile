ARG BASE_IMAGE=docker.io/library/ubuntu:26.04@sha256:f3d28607ddd78734bb7f71f117f3c6706c666b8b76cbff7c9ff6e5718d46ff64
####################################################################################################
# Builder image
# Initial stage which pulls prepares build dependencies and CLI tooling we need for our final image
# Also used as the image in CI jobs so needs all dependencies
####################################################################################################
FROM docker.io/library/golang:1.27.0@sha256:4013ae0f9e7994f8535c58c811f8f863fbed38b72e0d51e6592156f758d66146 AS builder

WORKDIR /tmp

# renovate: datasource=deb depName=git registryUrl=https://deb.debian.org/debian?suite=trixie&components=main,security&binaryArch=amd64
ARG GIT_APT_VERSION=1:2.47.3-0+deb13u1
# RUN echo 'deb http://archive.debian.org/debian buster-backports main' >> /etc/apt/sources.list

RUN apt-get update && apt-get install --no-install-recommends -y \
    openssh-server \
    nginx \
    unzip \
    fcgiwrap \
    git \
    git-lfs \
    make \
    wget \
    gcc \
    sudo \
    zip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# no need for this in monorepo controller
#COPY hack/install.sh hack/tool-versions.sh ./
#COPY hack/installers installers

#RUN ./install.sh helm && \
#    INSTALL_PATH=/usr/local/bin ./install.sh kustomize
# ./install.sh git-lfs

####################################################################################################
# Argo CD Base - used as the base for both the release and dev argocd images
####################################################################################################
FROM $BASE_IMAGE AS argocd-base

LABEL org.opencontainers.image.source="https://github.com/argoproj/argo-cd"

USER root

ENV ARGOCD_USER_ID=999 \
    DEBIAN_FRONTEND=noninteractive

# renovate: datasource=deb depName=git registryUrl=https://archive.ubuntu.com/ubuntu?suite=resolute&components=main,security&binaryArch=amd64
ARG GIT_APT_VERSION=1:2.53.0-1ubuntu1

RUN groupadd -g $ARGOCD_USER_ID argocd && \
    useradd -r -u $ARGOCD_USER_ID -g argocd argocd && \
    mkdir -p /home/argocd && \
    chown argocd:0 /home/argocd && \
    chmod g=u /home/argocd && \
    apt-get update && \
    apt-get dist-upgrade -y && \
    apt-get install --no-install-recommends -y \
    git=${GIT_APT_VERSION} tini ca-certificates gpg gpg-agent tzdata connect-proxy openssh-client && \
    apt-get install -y \
    git git-lfs tini gpg tzdata connect-proxy && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /usr/share/doc/*

COPY hack/gpg-wrapper.sh \
    hack/git-verify-wrapper.sh \
    entrypoint.sh \
    /usr/local/bin/
#COPY --from=builder /usr/local/bin/helm /usr/local/bin/helm
#COPY --from=builder /usr/local/bin/kustomize /usr/local/bin/kustomize
# COPY --from=builder /usr/local/bin/git-lfs /usr/local/bin/git-lfs

RUN git lfs install --system

# keep uid_entrypoint.sh for backward compatibility
RUN ln -s /usr/local/bin/entrypoint.sh /usr/local/bin/uid_entrypoint.sh

# support for mounting configuration from a configmap
WORKDIR /app/config/ssh
RUN touch ssh_known_hosts && \
    ln -s /app/config/ssh/ssh_known_hosts /etc/ssh/ssh_known_hosts

WORKDIR /app/config
RUN mkdir -p tls && \
    mkdir -p gpg/source && \
    mkdir -p gpg/keys && \
    chown argocd gpg/keys && \
    chmod 0700 gpg/keys

ENV USER=argocd

# Disable gRPC service config lookups via DNS TXT records to prevent excessive
# DNS queries for _grpc_config.<hostname> which can cause timeouts in dual-stack
# environments. This can be overridden via argocd-cmd-params-cm ConfigMap.
# See https://github.com/argoproj/argo-cd/issues/24991
ENV GRPC_ENABLE_TXT_SERVICE_CONFIG=false

USER $ARGOCD_USER_ID
WORKDIR /home/argocd

####################################################################################################
# Argo CD Build stage which performs the actual build of Argo CD binaries
####################################################################################################
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.27.0@sha256:4013ae0f9e7994f8535c58c811f8f863fbed38b72e0d51e6592156f758d66146 AS argocd-build

WORKDIR /go/src/github.com/argoproj/argo-cd

COPY go.* ./
RUN go mod download

# Perform the build
COPY . .
ARG TARGETOS \
    TARGETARCH
# These build args are optional; if not specified the defaults will be taken from the Makefile
ARG GIT_TAG \
    BUILD_DATE \
    GIT_TREE_STATE \
    GIT_COMMIT
RUN GIT_COMMIT=$GIT_COMMIT \
    GIT_TREE_STATE=$GIT_TREE_STATE \
    GIT_TAG=$GIT_TAG \
    BUILD_DATE=$BUILD_DATE \
    GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    make argocd-all

####################################################################################################
# Final image
####################################################################################################
FROM argocd-base
ENTRYPOINT ["/usr/bin/tini", "--"]
COPY --from=argocd-build /go/src/github.com/argoproj/argo-cd/dist/argocd* /usr/local/bin/

USER root
RUN ln -s /usr/local/bin/argocd /usr/local/bin/argocd-monorepo-repo-server && \
    ln -s /usr/local/bin/argocd /usr/local/bin/argocd-monorepo-controller

USER $ARGOCD_USER_ID
