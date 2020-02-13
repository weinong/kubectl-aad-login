package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

const (
	tokenType         = "Bearer"
	azureAuthProvider = "azure"

	defaultEnvironmentName = "AzurePublicCloud"

	envServicePrincipalClientID     = "AZURE_SERVICE_PRINCIPAL_CLIENT_ID"
	envServicePrincipalClientSecret = "AZURE_SERVICE_PRINCIPAL_CLIENT_SECRET"

	cfgClientID     = "client-id"
	cfgTenantID     = "tenant-id"
	cfgAccessToken  = "access-token"
	cfgRefreshToken = "refresh-token"
	cfgExpiresIn    = "expires-in"
	cfgExpiresOn    = "expires-on"
	cfgEnvironment  = "environment"
	cfgApiserverID  = "apiserver-id"
	cfgConfigMode   = "config-mode"
)

type tokenSource interface {
	Name() string
	Token() (adal.Token, error)
}

type tokenRefresher interface {
	Refresh() error
	ToCfg() map[string]string
}

type tokenSourceDeviceCode struct {
	environment azure.Environment
	clientID    string
	tenantID    string
	resourceID  string
	name        string
}

type tokenSourceServicePrincipal struct {
	environment  azure.Environment
	clientID     string
	clientSecret string
	tenantID     string
	resourceID   string
	name         string
}

type tokenSourceManualToken struct {
	source      tokenSource
	environment azure.Environment
	token       adal.Token
	clientID    string
	tenantID    string
	resourceID  string
	name        string
}

func (ts *tokenSourceServicePrincipal) Name() string {
	return "TokenSourceServicePrincipal"
}

func (ts *tokenSourceServicePrincipal) Token() (adal.Token, error) {
	emptyToken := adal.Token{}
	oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(ts.environment.ActiveDirectoryEndpoint, ts.tenantID, nil)
	if err != nil {
		return emptyToken, fmt.Errorf("building the OAuth configuration without api-version for device code authentication: %v", err)
	}
	callback := func(t adal.Token) error {
		return nil
	}
	spt, err := adal.NewServicePrincipalToken(*oauthConfig, ts.clientID, ts.clientSecret, ts.resourceID, callback)
	if err != nil {
		return emptyToken, fmt.Errorf("creating new service principal for token refresh: %v", err)
	}

	err = spt.Refresh()
	if err != nil {
		return emptyToken, err
	}
	return spt.Token(), nil
}

func newTokenSourceServicePrincipal(environment azure.Environment, clientID, clientSecret, tenantID, resourceID string) (tokenSource, error) {
	if clientID == "" {
		return nil, errors.New("clientID is empty")
	}
	if clientSecret == "" {
		return nil, errors.New("clientSecret is empty")
	}
	if tenantID == "" {
		return nil, errors.New("tenantID is empty")
	}
	if resourceID == "" {
		return nil, errors.New("resourceID is empty")
	}
	return &tokenSourceServicePrincipal{
		environment:  environment,
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		resourceID:   resourceID,
	}, nil
}

func (ts *tokenSourceDeviceCode) Name() string {
	return "TokenSourceDeviceCode"
}

func (ts *tokenSourceDeviceCode) Token() (adal.Token, error) {
	emptyToken := adal.Token{}
	oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(ts.environment.ActiveDirectoryEndpoint, ts.tenantID, nil)
	if err != nil {
		return emptyToken, fmt.Errorf("building the OAuth configuration without api-version for device code authentication: %v", err)
	}
	client := &autorest.Client{}
	deviceCode, err := adal.InitiateDeviceAuth(client, *oauthConfig, ts.clientID, ts.resourceID)
	if err != nil {
		return emptyToken, fmt.Errorf("initialing the device code authentication: %v", err)
	}

	_, err = fmt.Fprintln(os.Stderr, *deviceCode.Message)
	if err != nil {
		return emptyToken, fmt.Errorf("prompting the device code message: %v", err)
	}

	token, err := adal.WaitForUserCompletion(client, deviceCode)
	if err != nil {
		return emptyToken, fmt.Errorf("waiting for device code authentication to complete: %v", err)
	}

	return *token, nil
}

func newTokenSourceDeviceCode(environment azure.Environment, clientID string, tenantID string, resourceID string) (tokenSource, error) {
	if clientID == "" {
		return nil, errors.New("clientID is empty")
	}
	if tenantID == "" {
		return nil, errors.New("tenantID is empty")
	}
	if resourceID == "" {
		return nil, errors.New("resourceID is empty")
	}
	return &tokenSourceDeviceCode{
		environment: environment,
		clientID:    clientID,
		tenantID:    tenantID,
		resourceID:  resourceID,
	}, nil
}

func (ts *tokenSourceManualToken) ToCfg() map[string]string {
	return map[string]string{
		cfgAccessToken:  ts.token.AccessToken,
		cfgRefreshToken: ts.token.RefreshToken,
		cfgEnvironment:  ts.environment.Name,
		cfgClientID:     ts.clientID,
		cfgTenantID:     ts.tenantID,
		cfgApiserverID:  ts.resourceID,
		cfgExpiresIn:    string(ts.token.ExpiresIn),
		cfgExpiresOn:    string(ts.token.ExpiresOn),
		cfgConfigMode:   "1",
	}
}

func (ts *tokenSourceManualToken) Name() string {
	return "TokenSourceManualToken"
}

func (ts *tokenSourceManualToken) Token() (adal.Token, error) {
	emptyToken := adal.Token{}
	oauthConfig, err := adal.NewOAuthConfigWithAPIVersion(ts.environment.ActiveDirectoryEndpoint, ts.tenantID, nil)
	if err != nil {
		return emptyToken, fmt.Errorf("building the OAuth configuration without api-version for token refresh: %v", err)
	}

	callback := func(t adal.Token) error {
		return nil
	}
	spt, err := adal.NewServicePrincipalTokenFromManualToken(
		*oauthConfig,
		ts.clientID,
		ts.resourceID,
		ts.token,
		callback)
	if err != nil {
		return emptyToken, fmt.Errorf("creating new service principal for token refresh: %v", err)
	}

	err = spt.Refresh()
	if err != nil {
		return emptyToken, err
	}
	return spt.Token(), nil
}

func (ts *tokenSourceManualToken) Refresh() error {
	// if token is empty, invoke token source
	if ts.token.IsZero() {
		token, err := ts.source.Token()
		if err != nil {
			return err
		}
		ts.token = token
		fmt.Fprintln(os.Stderr, "obtained new token")
	}

	// if token has not expired, no need to refresh
	if !ts.token.IsExpired() {
		fmt.Fprintln(os.Stderr, "token is not expired. no need to refresh")
		return nil
	}

	token, err := ts.Token()
	// if refresh fails, refresh token may have expired, invoke token source again
	if err != nil {
		fmt.Fprintf(os.Stderr, "refreshing token failed. fall back to inner source %s\n", ts.source.Name())
		token, err := ts.source.Token()
		if err != nil {
			return err
		}
		ts.token = token
	} else {
		fmt.Fprintln(os.Stderr, "token is refreshed")
		ts.token = token
	}

	return nil
}

func newTokenSourceManualToken(environment azure.Environment, clientID, tenantID, resourceID string, token adal.Token, tokenSource tokenSource) (*tokenSourceManualToken, error) {
	if clientID == "" {
		return nil, errors.New("clientID is empty")
	}
	if tenantID == "" {
		return nil, errors.New("tenantID is empty")
	}
	if resourceID == "" {
		return nil, errors.New("resourceID is empty")
	}
	return &tokenSourceManualToken{
		source:      tokenSource,
		token:       token,
		environment: environment,
		clientID:    clientID,
		tenantID:    tenantID,
		resourceID:  resourceID,
	}, nil
}

func newTokenRefresher(cfg map[string]string, useSPN, forceRefresh bool) (tokenRefresher, error) {
	var (
		tenantID     string
		clientID     string
		clientSecret string
		resourceID   string
		innerTS      tokenSource
		env          azure.Environment
		err          error
		ok           bool
	)

	tenantID = cfg[cfgTenantID]
	if tenantID == "" {
		return nil, fmt.Errorf("no tenant ID in cfg: %s", cfgTenantID)
	}
	resourceID = cfg[cfgApiserverID]
	if resourceID == "" {
		return nil, fmt.Errorf("no apiserver ID in cfg: %s", cfgApiserverID)
	}
	environment := cfg[cfgEnvironment]
	if environment == "" {
		environment = defaultEnvironmentName
	}
	env, err = azure.EnvironmentFromName(environment)
	if err != nil {
		return nil, err
	}

	if useSPN {
		clientID, ok = os.LookupEnv(envServicePrincipalClientID)
		if !ok {
			return nil, fmt.Errorf("cannot find %s environment variable", envServicePrincipalClientID)
		}
		clientSecret, ok = os.LookupEnv(envServicePrincipalClientSecret)
		if !ok {
			return nil, fmt.Errorf("cannot find %s environment variable", envServicePrincipalClientSecret)
		}
		innerTS, err = newTokenSourceServicePrincipal(env, clientID, clientSecret, tenantID, resourceID)
		if err != nil {
			return nil, err
		}
	} else {
		clientID = cfg[cfgClientID]
		if clientID == "" {
			return nil, fmt.Errorf("no client ID in cfg: %s", cfgClientID)
		}
		innerTS, err = newTokenSourceDeviceCode(env, clientID, tenantID, resourceID)
		if err != nil {
			return nil, err
		}
	}

	accessToken := cfg[cfgAccessToken]
	refreshToken := cfg[cfgRefreshToken]
	expiresIn := cfg[cfgExpiresIn]
	expiresOn := cfg[cfgExpiresOn]

	var token adal.Token
	// no need to check refresh token as it may be empty when spn auth is used
	if !forceRefresh && accessToken != "" && expiresIn != "" && expiresOn != "" {
		token = adal.Token{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    json.Number(expiresIn),
			ExpiresOn:    json.Number(expiresOn),
			NotBefore:    json.Number(expiresOn),
			Resource:     resourceID,
			Type:         tokenType,
		}
	}

	return newTokenSourceManualToken(env, clientID, tenantID, resourceID, token, innerTS)
}
