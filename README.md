# Photon container with agent process

## Overview

This is a container image that manages [Photon](https://github.com/komoot/photon) in a container, which includes an agent written in Go that manages the Photon process.
When the agent receives an HTTP request, it stops the Photon process, downloads the data, and restarts the Photon process.

## Motivation

The index data of Photon is large (over 100GiB).
When downloading and extracting the data within the container, it temporarily requires **twice the storage capacity of the extracted data**.
However, once the extraction is complete, the archive is deleted, meaning only half the storage capacity is actually utilized.

In a home lab server environment, resources are often limited, and allocating storage that is not typically used should be avoided.
This is especially true when managing Photon on a Kubernetes cluster, where PersistentVolumes can be expanded but are difficult to shrink.

On the other hand, it is relatively easy to temporarily add storage to client devices.
For example, using an external SSD can provide terabytes of temporary storage with minimal effort.

By downloading and extracting the Photon data on a client device, transferring the extracted `tar` archive to the Photon container, and then extracting the data within the container, Photon can be operated with minimal storage requirements.
This approach necessitates an additional process, not included in Photon itself, to handle HTTP requests and manage data extraction.

## Features

- Download the index data from the internet
- Updating the Photon index
    - Sequential update mode
        - Stop the Photon process, delete the old index, extract the new index, and start the Photon process.
    - Parallel update mode
        - Extract the new index while the Photon process is running, and then stop the Photon process, replace the old index with the new one, and start the Photon process.
- Monitoring the photon index updates
    - Expose as a Prometheus metric

**Automated updating of the Photon index is not supported**. It is up to the user to decide when to update the index data.

## Usage

See [examples](./examples) directory for examples.

### Updating the Photon index

#### Server-side update

Currently, the server-side update initiates asynchronously. You can check the status of the update process via the `/migrate/status` endpoint.

If you want to know the details of the update process, you can check the log of the container.

```sh
PHOTON_AGENT_URL=http://localhost:8080
curl -X POST ${PHOTON_AGENT_URL}/migrate/download
```

#### Client-side update

Install `photon-db-updater` on your client device. It provides the way to download and verify MD5 checksum.

```sh
go install github.com/pddg/photon-container/cmd/photon-db-updater@latest

# or

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/aarch64/arm64/' | sed 's/x86_64/amd64/')
wget -O photon-db-updater \
    https://github.com/pddg/photon-container/releases/latest/download/photon-db-updater-${OS}-${ARCH}
```

Then, download the archive.

> [!WARNING]
> More than 100 GiB of data will be downloaded from the internet. Depending on your network speed, the download may take a long time. 

```sh
photon-db-updater \
    -download-only \
    -download-to photon-db.tar.bz2
```

Decompress the archive via `pbzip2`.

> [!WARNING]
> About 200 GiB of data will be extracted from the archive. Depending on your CPU and memory, this process may take a long time.

```sh
pbzip2 -d ./photon-db.tar.bz2
```

Upload the decompressed tar to your photon instance.

```sh
PHOTON_AGENT_URL=http://localhost:8080 \
photon-db-updater \
    -archive photon-db.tar \
    -no-compressed \
    -photon-agent-url ${PHOTON_AGENT_URL}
```

## Configuration

Configuration is done via environment variables. The following environment variables are available:

| Environment Variable | Description | Default Value |
|----------------------|-------------|---------------|
| `PHOTON_AGENT_DATABASE_URL` | The URL of the Photon database. | `https://download1.graphhopper.com/public/experimental/` |
| `PHOTON_AGENT_DATABASE_COUNTRY_CODE` | The country code for the Photon index data. The code can be found from [here](https://download1.graphhopper.com/public/experimental/extracts/by-country-code/). | `` (full index data) |
| `PHOTON_AGENT_UPDATE_STRATEGY` | The update strategy for the Photon index. Can be `sequential` or `parallel`. | `sequential` |
| `PHOTON_AGENT_DOWNLOAD_SPEED_LIMIT` | The speed limit for downloading the Photon index data. e.g. `10MB` | (no limit) |
| `PHOTON_AGENT_IO_SPEED_LIMIT` | The speed limit for storage I/O operations. e.g. `10MB` | (no limit) |
| `PHOTON_AGENT_LOG_LEVEL` | The log level for the Photon agent. Can be `debug`, `info`, `warn`, or `error`. | `info` |
| `PHOTON_AGENT_LOG_FORMAT` | The log format for the Photon agent. Can be `text` or `json`. | `json` |

## Update Strategy Comparison

The update strategy is determined by the combination of the `PHOTON_AGENT_UPDATE_STRATEGY` and how the archive is downloaded and extracted. `sequential` and `parallel` are the two update strategies, while `server` and `client` refer to where the archive is downloaded and decompressed.

### `sequential` + `server`

- All processes are done on the server side.
    1. Stop the Photon process
    2. Delete the old index
    3. Download the new index
    4. Extract the new index
    5. Start the Photon process
- Pros
    - Simple
    - No need to download the archive on the client side
- Cons
    - Requires twice the storage capacity of the extracted data
        - Decompressed archive + New index
        - About 400GiB of storage is required
    - Long downtime
        - Download time + Extracting time
    - Large computation power required on the server side
        - Decompressing the archive is CPU intensive

### `sequential` + `client`

- Download the archive on the client side and extract it on the server side.
    1. Download and decompress the new index on client
    2. Transfer the extracted data to the server
    3. Agent initiates the update process
        1. Stop the Photon process
        2. Delete the old index
        3. Extract the new index
        4. Start the Photon process
- Pros
    - Minimal storage capacity required
        - Only the size of the extracted data is required
        - About 200GiB of storage is required
    - Relatively short downtime (compared to `sequential` + `server`)
        - Transfer time + Extracting time
        - The transfer time is usually shorter than the download time (typically, local network speed is faster than the internet speed)
- Cons
    - Requires a client device with sufficient storage capacity
    - Requires some tools to download and extract the archive on the client side
    - Large computation power required on the client side
        - Decompressing the archive is CPU intensive

### `parallel` + `server`

- All processes are done on the server side.
    1. Download the new index
    2. Extract the new index
    3. Stop the Photon process
    4. Replace the old index with new one
    5. Start the Photon process
- Pros
    - Minimal downtime
        - Only the time to stop and start the Photon process
    - No need to download the archive on the client side
- Cons
    - Requires three times the storage capacity of the extracted data
        - Decompressed archive + New index + Old index
        - About 500GiB of storage is required
    - Large computation power required on the server side
        - Decompressing the archive is CPU intensive

### `parallel` + `client`

- Download the archive on the client side and extract it on the server side.
    1. Download the new index on client
    2. Decompress the archive on client
    3. Transfer the extracted data to the server
    4. Agent initiates the update process
        1. Extract the new index
        2. Stop the Photon process
        3. Replace the old index with new one
        4. Start the Photon process
- Pros
    - Minimal downtime
        - Only the time to stop and start the Photon process
    - Relatively less storage capacity required (compared to `sequential` + `server`)
        - New index + Old index
        - About 400GiB of storage is required
- Cons
    - Requires a client device with sufficient storage capacity
    - Requires some tools to download and extract the archive on the client side
    - Large computation power required on the client side
        - Decompressing the archive is CPU intensive

### Which one to choose?

If you have enough storage capacity and computing resource on the server side, `parallel` + `server` is the best way to update the index.

If your server has limited storage capacity and computing resource, `sequential` + `client` is the best way to update the index.

## Acknowledgements

- [rtuszik/photon-docker](https://github.com/rtuszik/photon-docker)
    - Downloading and updating the Photon index process is based on this repository.
