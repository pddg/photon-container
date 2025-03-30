# Deploy Photon via Docker Compose

> [!WARNING]
> This example is not tested.

## Prerequisites

- Ensure you have Docker and Docker Compose installed on your system.
- Clone this repository to your local machine.

## Steps to Deploy

> [!NOTE]
> By default, the environment variable `PHOTON_AGENT_DATABASE_COUNTRY_CODE` is set, which limits the download to data for a few countries with smaller index sizes.
> You can find country codes from [here](https://download1.graphhopper.com/public/experimental/extracts/by-country-code/).

> [!WARNING]
> If you plan to use the full-size index, ensure that the volume has enough capacity to accommodate both the compressed archive and the extracted data. This requires at least 350 GiB of storage space.

1. Navigate to the `examples/compose` directory:
    ```bash
    cd /path/to/photon-container/examples/compose
    ```
2. Start the services using Docker Compose:
   ```bash
   docker compose up -d
   ```
4. Download and extract the index data from internet
   ```bash
   curl -X POST http://localhost:8080/migrate/download
   ```

## Stopping the Services

To stop and remove the containers, run:
```bash
docker compose down
```
