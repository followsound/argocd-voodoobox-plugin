# build stage
FROM golang:1 AS build

LABEL maintainer="Sam Clark <sam@followsound.co.uk>"
LABEL org.opencontainers.image.source = "https://github.com/followsound/argocd-voodoobox-plugin"
LABEL org.opencontainers.image.description = "An Argo CD plugin to decrypt strongbox encrypted files and build Kubernetes resources"

ENV \
  STRONGBOX_VERSION=1.0.1 \
  KUSTOMIZE_VERSION=v4.5.5 \
  HELM_VERSION=v3.11.0

RUN os=$(go env GOOS) && arch=$(go env GOARCH) \
  && curl -Ls https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_${os}_${arch}.tar.gz \
    | tar xz -C /usr/local/bin/ \
  && chmod +x /usr/local/bin/kustomize \
  && curl -Ls https://github.com/uw-labs/strongbox/releases/download/v${STRONGBOX_VERSION}/strongbox_${STRONGBOX_VERSION}_${os}_${arch} \
    > /usr/local/bin/strongbox \
  && chmod +x  /usr/local/bin/strongbox \
  && curl -O https://get.helm.sh/helm-${HELM_VERSION}-${os}-${arch}.tar.gz \
  && tar -xf helm-${HELM_VERSION}-${os}-${arch}.tar.gz \
  && cp ${os}-${arch}/helm /usr/local/bin/helm \
  && chmod +x /usr/local/bin/helm

ADD . /app

WORKDIR /app


RUN go test -v -cover ./... \
    && go build -ldflags='-s -w' -o /argocd-voodoobox-plugin .

# final stage
# argocd requires that sidecar container is running as user 999
FROM ubuntu:22.04

USER root

ENV ARGOCD_USER_ID=999

RUN groupadd -g $ARGOCD_USER_ID argocd && \
    useradd -r -u $ARGOCD_USER_ID -g argocd argocd && \
    apt-get update && \
    apt-get -y upgrade && \
  apt-get install -y git git-lfs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=build \
  /usr/local/bin/kustomize \
  /usr/local/bin/strongbox \
  /usr/local/bin/helm \
  /argocd-voodoobox-plugin \
  /usr/local/bin/

ENV USER=argocd

USER $ARGOCD_USER_ID
