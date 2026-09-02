# Install on Kubernetes

The official Helm chart deploys the bundled Nginx UI container, including its
own Nginx instance. It is an application deployment, not a Kubernetes Ingress
Controller.

## Requirements

- Kubernetes 1.25 or newer
- Helm 3
- A default StorageClass, or existing PersistentVolumeClaims
- At least 2 GiB of persistent storage for the default configuration

Nginx UI is a single-writer workload. The chart enforces one replica and uses a
`Recreate` deployment strategy so two pods never write the same configuration
or SQLite database concurrently.

## Install from the self-hosted repository

```shell
helm repo add nginx-ui https://cloud.nginxui.com/helm
helm repo update nginx-ui
helm install nginx-ui nginx-ui/nginx-ui \
  --namespace nginx-ui \
  --create-namespace
```

Wait for the Deployment and both PersistentVolumeClaims:

```shell
kubectl get deployment,pod,pvc -n nginx-ui
kubectl rollout status deployment/nginx-ui -n nginx-ui
```

For local access, forward the HTTP Service port:

```shell
kubectl port-forward -n nginx-ui service/nginx-ui 8080:80
```

Open `http://127.0.0.1:8080`. Read the one-time installation secret from the
persistent Nginx UI data volume:

```shell
kubectl exec -n nginx-ui deployment/nginx-ui -- \
  cat /etc/nginx-ui/.install_secret
```

## Ingress

The chart can create a standard Kubernetes Ingress that forwards to the bundled
Nginx HTTP port:

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: nginx-ui.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: nginx-ui-tls
      hosts:
        - nginx-ui.example.com
```

Save the values as `values.yaml` and install with `--values values.yaml`. The
Ingress Controller and TLS Secret must already exist in the cluster.

## Persistence

The chart creates and retains these claims by default:

| Claim | Container path | Purpose |
| --- | --- | --- |
| `nginx` | `/etc/nginx` | Bundled Nginx configuration |
| `nginxUI` | `/etc/nginx-ui` | Database, application settings, and install secret |

An optional `www` claim mounts at `/var/www`. Set a StorageClass per claim with
`persistence.<name>.storageClass`, or reuse a pre-provisioned claim with
`persistence.<name>.existingClaim`.

Claims created by the chart carry `helm.sh/resource-policy: keep` by default.
Uninstalling the release therefore leaves user configuration and the database
intact. Delete retained claims manually only after backing up their contents.

## Upgrade

```shell
helm repo update nginx-ui
helm upgrade nginx-ui nginx-ui/nginx-ui \
  --namespace nginx-ui \
  --reuse-values
```

The release pipeline tests a fresh install, writes a marker to the Nginx UI data
claim, performs an upgrade that recreates the pod, and verifies that the marker
and `/healthz` endpoint remain available.
