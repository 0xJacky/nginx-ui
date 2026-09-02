# Nginx UI Helm Chart

This chart deploys one Nginx UI instance together with the Nginx bundled in the
official container image. It is intentionally single-replica because the Nginx
configuration and Nginx UI database are stored on writable persistent volumes.

The chart is not a Kubernetes Ingress controller and does not watch or modify
Kubernetes Ingress resources. The optional `ingress` value only exposes the
Nginx UI web interface through an existing Ingress controller.

## Install

```shell
helm repo add nginx-ui https://cloud.nginxui.com/helm
helm repo update
helm install nginx-ui nginx-ui/nginx-ui --namespace nginx-ui --create-namespace
```

Read the one-time installation secret after the pod starts:

```shell
kubectl exec -n nginx-ui deploy/nginx-ui -- cat /etc/nginx-ui/.install_secret
```

## Persistent data

The chart creates retained PVCs for `/etc/nginx` and `/etc/nginx-ui` by default.
The `/var/www` PVC is optional. Set `existingClaim` for any volume to reuse a
pre-created claim. Retained PVCs are not deleted by `helm uninstall`; remove
them manually only after backing up the data.

## Important values

| Value | Default | Description |
| --- | --- | --- |
| `image.repository` | `uozi/nginx-ui` | Official container repository |
| `image.tag` | chart `appVersion` | Container tag |
| `service.type` | `ClusterIP` | Kubernetes Service type |
| `ingress.enabled` | `false` | Expose the UI through an existing Ingress controller |
| `persistence.nginx.size` | `1Gi` | Nginx configuration volume size |
| `persistence.nginxUI.size` | `1Gi` | Nginx UI data volume size |
| `persistence.www.enabled` | `false` | Persist `/var/www` |

The values schema enforces `replicaCount: 1`. Horizontal scaling is not
supported because multiple instances must not concurrently mutate the same
Nginx configuration or embedded database.
