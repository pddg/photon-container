FROM golang:1.24.1 AS builder

WORKDIR /workdir

COPY go.mod go.sum /workdir

RUN go mod download

COPY . /workdir

RUN make all 

ARG PHOTON_VERSION
ARG PHOTON_SHA256SUM

ADD https://github.com/komoot/photon/releases/download/${PHOTON_VERSION}/photon-opensearch-${PHOTON_VERSION}.jar /photon/photon.jar

RUN echo "${PHOTON_SHA256SUM}  /photon/photon.jar" | sha256sum -c -

FROM gcr.io/distroless/java21-debian12:nonroot

ARG PHOTON_VERSION
ARG GIT_SHA

LABEL maintainer="github.com/pddg" \
      app.pddg.version.photon="${PHOTON_VERSION}" \
      org.opencontainers.image.revision="${GIT_SHA}" \
      org.opencontainers.image.source="https://github.com/pddg/photon-container"

USER nonroot

COPY --chown=nonroot:nonroot --from=builder /workdir/build/photon-wrapper /usr/local/bin/photon-wrapper
COPY --chown=nonroot:nonroot --from=builder /workdir/build/photon-db-updater /usr/local/bin/photon-db-updater

WORKDIR /photon

COPY --chown=nonroot:nonroot --from=builder /photon/photon.jar /photon/photon.jar

ENTRYPOINT ["/usr/local/bin/photon-wrapper"]
