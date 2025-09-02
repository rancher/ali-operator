# Image that provides cross compilation tooling.
ARG BCI_VERSION=15.7
FROM --platform=$BUILDPLATFORM rancher/mirrored-tonistiigi-xx:1.5.0 AS xx

FROM registry.suse.com/bci/bci-base:${BCI_VERSION} AS base
RUN sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd
RUN useradd --uid 1007 ali-operator

FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY ./controller ./controller
COPY ./pkg ./pkg
COPY ./main.go ./main.go

# Copy xx scripts to your build stage
COPY --from=xx / /

ARG TARGETPLATFORM
ARG COMMIT
ARG VERSION
ENV CGO_ENABLED=0
RUN xx-go build -ldflags \
           "-X github.com/rancher/ali-operator/pkg/version.GitCommit=${COMMIT} \
            -X github.com/rancher/ali-operator/pkg/version.Version=${VERSION}" \
       -o /ali-operator && \
       xx-verify /ali-operator

FROM registry.suse.com/bci/bci-micro:${BCI_VERSION}
COPY --from=base /etc/passwd /etc/passwd
COPY --from=base /etc/shadow /etc/shadow
COPY --from=builder /ali-operator /usr/bin/ali-operator

RUN rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*

ENV KUBECONFIG="/home/ali-operator/.kube/config"
ENV SSL_CERT_DIR="/etc/rancher/ssl"

COPY package/entrypoint.sh /usr/bin
RUN chmod +x /usr/bin/entrypoint.sh

RUN mkdir -p /etc/rancher/ssl && \
    chown -R ali-operator /etc/rancher/ssl

USER 1007
ENTRYPOINT ["entrypoint.sh"]
