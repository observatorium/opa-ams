FROM golang:1.14.1-alpine3.11 as builder

RUN apk add --update --no-cache ca-certificates tzdata git make bash && update-ca-certificates

ADD . /opt
WORKDIR /opt

RUN git update-index --refresh; make opa-ams

FROM alpine:3.10 as runner

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /opt/opa-ams /bin/opa-ams

ARG BUILD_DATE
ARG VERSION
ARG VCS_REF
ARG DOCKERFILE_PATH

LABEL vendor="Observatorium" \
    name="observatorium/opa-ams" \
    description="OPA-AMS proxy" \
    io.k8s.display-name="observatorium/opa-ams" \
    io.k8s.description="OPA-AMS proxy" \
    maintainer="Observatorium <team-monitoring@redhat.com>" \
    version="$VERSION" \
    org.label-schema.build-date=$BUILD_DATE \
    org.label-schema.description="OPA-AMS proxy" \
    org.label-schema.docker.cmd="docker run --rm observatorium/opa-ams" \
    org.label-schema.docker.dockerfile=$DOCKERFILE_PATH \
    org.label-schema.name="observatorium/opa-ams" \
    org.label-schema.schema-version="1.0" \
    org.label-schema.vcs-branch=$VCS_BRANCH \
    org.label-schema.vcs-ref=$VCS_REF \
    org.label-schema.vcs-url="https://github.com/observatorium/opa-ams" \
    org.label-schema.vendor="observatorium/opa-ams" \
    org.label-schema.version=$VERSION

ENTRYPOINT ["/bin/opa-ams"]
