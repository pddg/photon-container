FROM mirror.gcr.io/debian:stable-slim AS builder

ARG PHOTON_VERSION
ARG PHOTON_SHA256SUM

ADD https://github.com/komoot/photon/releases/download/${PHOTON_VERSION}/photon-opensearch-${PHOTON_VERSION}.jar /photon/photon.jar

RUN echo "${PHOTON_SHA256SUM}  /photon/photon.jar" | sha256sum -c -

FROM gcr.io/distroless/java21-debian12:nonroot

ARG TARGETOS
ARG TARGETARCH
ARG PHOTON_VERSION
ARG GIT_SHA

LABEL maintainer="github.com/pddg" \
      app.pddg.version.photon="${PHOTON_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}" \
      org.opencontainers.image.source="https://github.com/pddg/photon-container"

USER nonroot

COPY ./build/photon-db-updater-${TARGETOS}-${TARGETARCH} /usr/local/bin/photon-db-updater
COPY ./build/photon-agent-${TARGETOS}-${TARGETARCH} /usr/local/bin/photon-agent

WORKDIR /photon

COPY --chown=nonroot:nonroot --from=builder /photon/photon.jar /photon/photon.jar

ENTRYPOINT ["/usr/local/bin/photon-agent"]
