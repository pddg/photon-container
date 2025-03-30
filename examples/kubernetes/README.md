# Deploy photon on kubernetes

## Test on kind

```bash
kind create cluster --name photon
kubectl create ns photon
kustomize build . | kubectl apply -f - -n photon
# wait for photon-0 to be ready
kubectl wait pod photon-0 --for=condition=Ready -n photon
```

Started container has no data. You can call API to download data from the internet.

> [!WARNING]
> The full archive size exceeds 110 GiB. Ensure your machine has sufficient storage.  
> After extraction, the archive expands to approximately 190 GiB.
> Since both the archive and its extracted contents consume storage temporarily during the process, it is recommended to have a minimum of 400 GiB of available volume.
> This accounts for up to twice the size of the extracted data due to the simultaneous storage of the archive and its expanded contents.

> [!NOTE]
> If you want to test with small data, you can specify `PHOTON_AGENT_DATABASE_COUNTRY_CODE` .
> Example manifest uses `PHOTON_AGENT_DATABASE_COUNTRY_CODE: ad`.
> You can find country codes from [here](https://download1.graphhopper.com/public/experimental/extracts/by-country-code/).

```bash
# Port forward to management API
kubectl port-forward svc/photon 8080:8080 -n photon

# @Another terminal
# Download latest data from the internet
curl -X POST http://localhost:8080/migrate/download
```

Wait until migration status is `migrated`.

```
❯ curl -X GET 'http://localhost:8080/migrate/status'
{"state":"migrated","version":"2025-03-16T06:31:27Z"}
```

Then you can access the data.

```bash
# Port forward to Photon API
kubectl port-forward svc/photon 2322:2322 -n photon

# @Another terminal
# Reverse geocoding
curl -sS -X GET 'http://localhost:2322/reverse?lat=42.508004&lon=1.529161'
```

## Downloading/Decompressing the archive outside of the Pod

During decompression, the original archive file cannot be deleted. This temporarily consumes storage equivalent to the combined size of the compressed and decompressed archives.

While Kubernetes allows you to expand Persistent Volumes during operation, reducing their size is challenging. Therefore, it is advisable to avoid expanding Persistent Volumes solely for temporary storage needs.

To address this, you can prepare the archive in your laptop or create a temporary Pod with an ephemeral volume mounted. This Pod can be used to decompress and extract the tar archive, and the extracted data can then be uploaded via the API.

> [!WARNING]
> Ensure that your Kubernetes cluster's StorageClass supports Generic Ephemeral Volumes. Using `emptyDir` is not recommended as it may exhaust the node's volume capacity.

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: bastion
spec:
  containers:
    - name: bastion
      image: ubuntu:24.04
      command:
        - sleep
        - infinity
      volumeMounts:
        - name: workdir
          mountPath: /workdir
  volumes:
    - name: workdir
      ephemeral:
        volumeClaimTemplate:
          spec:
            accessModes: [ "ReadWriteOnce" ]
            resources:
              requests:
                storage: 350Gi
```

The combined size of the compressed archive and the decompressed archive temporarily consumes storage.
To extract the full-size index data, prepare a volume of approximately 350 Gi to 400 Gi.

First, install `tar` and `pbzip2`. `pbzip2` is highly recommended to decompress the data.

```sh
kubectl exec -it bastion -- bash

# @bastion Pod
apt-get update && apt-get install -y wget tar pbzip2 --no-install-recommends
```

Second, install `photon-db-updater`. It provides the way to download and verify MD5 checksum.

```sh
# @bastion Pod
ARCH=$(uname -m | sed 's/aarch64/arm64/' | sed 's/x86_64/amd64/')
wget -O /bin/photon-db-updater \
  https://github.com/pddg/photon-container/releases/latest/download/photon-db-updater-linux-$ARCH
```

Then, download archive.

> [!WARNING]
> More than 100 GiB of data will be downloaded from the internet. Depending on your network speed, the download may take a long time. If the `kubectl exec` session is disconnected, the download may be interrupted.
> You should create Job resource to download the archive, if you want to run it in the background.

```sh
kubectl exec -it bastion -- bash

# @bastion Pod
cd /workdir
photon-db-updater \
  -download-only \
  -download-to ./photon-db.tar.bz2
```

Decompress the archive via `pbzip2`.

> [!WARNING]
> `pbzip2` operates in a multi-threaded manner and, by default, consumes the maximum resources allocated to the Pod during decompression. If necessary, adjust the resource limits to avoid becoming a noisy neighbor.

```sh
cd /workdir

# add `-k` if you want not to delete compressed archive.
pbzip2 -d ./photon-db.tar.bz2
```

Upload the decompressed tar to the photon Pod.

> [!WARNING]
> Depending on the communication speed between Pods in the Kubernetes cluster and the write speed to storage, this process may take a significant amount of time.

```sh
cd /workdir

photon-db-updater \
  -archive ./photon-db.tar \
  -no-compressed \
  -photon-agent-url http://api.photon.svc:8080/ \
  --wait
```

After completing this command, ensure that Photon is running and verify that reverse geocoding works as expected.

```sh
# @bastion Pod
curl -X GET http://api.photon.svc/status
curl -X GET 'http://api.photon.svc/reverse?lat=42.508004&lon=1.529161'
```
