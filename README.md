![Build on Push](https://github.com/weinong/kubectl-aad-login/workflows/Build%20on%20Push/badge.svg?branch=master)

# kubectl-aad-login
It is a [kubectl plugin](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/) supporting various OAuth login flows on Azure AD which are not currently supported in `kubectl`. 
It populates the kubeconfig file with acquired AAD token. It will refresh access token when the access token has expired.
Currently, it supports:
* device code flow with fix to https://github.com/kubernetes/kubernetes/issues/86410 such that `audience` claim does not have `spn:` prefix (supports AKS AADv1 and v2, this change is required for AKS AADv2 unless you are on kubectl versions TBD...)
* non-interactive login using service principal credential (supports AKS AADv2 only)
* non-interactive login using user principal credential (supports AKS AADv1 and v2)

The environment being tested is AKS AAD and AKS AADv2 (public preview in March 2020)

## Install
Download the plugin https://github.com/weinong/kubectl-aad-login/releases/download/v0.0.1/kubectl-aad-login.zip

Copy out the binary to directory under search path

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

> Note: it will only work on AKS AAD v2

```sh
export AAD_SERVICE_PRINCIPAL_CLIENT_ID=<Service-Principal-Client-ID>
export AAD_SERVICE_PRINCIPAL_CLIENT_SECRET=<Service-Principal-Client-Secret>

kubectl aad login --service-principal
```

### User Principal login
non-interactive login using user principal credential. It uses [Resource Owner Password Credential flow](https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-oauth-ropc) 

> Note: ROPC is not supported in hybrid identity federation scenarios (for example, Azure AD and ADFS used to authenticate on-premises accounts). If users are full-page redirected to an on-premises identity providers, Azure AD is not able to test the username and password against that identity provider. Pass-through authentication is supported with ROPC, however.
> It also does not work when MFA policy is enabled
> Personal accounts that are invited to an Azure AD tenant can't use ROPC.

```sh
export AAD_USER_PRINCIPAL_USERNAME=foo@bar.com
export AAD_USER_PRINCIPAL_PASSWORD=<password>

kubectl aad login --user-principal
```

### force refresh

Append `--force` to disregard refresh token and always initiates login flow
