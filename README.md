# kubectl-aad-login
It is a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) supporting various OAuth login flows on Azure AD which are not currently supported in `kubectl`. 
It populates the kubeconfig file with acquired AAD token. It will refresh access token when the access token has expired.
Currently, it supports:
* device code flow with fix to https://github.com/kubernetes/kubernetes/issues/86410 such that `audience` claim does not have `spn:` prefix
* non-interactive login using service principal credential

## Build
```sh
GO111MODULE="on" go build cmd/kubectl-aad-login.go
mv kubectl-aad-login /path/to/go/bin
```

## How to use

### Device code flow
It's similar to current kubectl implementation except that the resulting AAD token will have proper `audience` claim with "spn:" prefix
It addresses https://github.com/kubernetes/kubernetes/issues/86410

```sh
kubectl aad login
```

### Service Principal login
non-interactive login using service principal credential

```sh
export AZURE_SERVICE_PRINCIPAL_CLIENT_ID=<Service-Principal-Client-ID>
export AZURE_SERVICE_PRINCIPAL_CLIENT_SECRET=<Service-Principal-Client-Secret>

kubectl aad login --service-principal
```
