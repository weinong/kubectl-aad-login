package msal

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"

	"github.com/Azure/go-autorest/autorest/adal"
)

const (
	oAuthGrantTypeRefreshToken       = "refresh_token"
	oAuthGrantTypeResourceOwnerToken = "password"
	number                           = "v1.0.0"
	contentType                      = "Content-Type"
	mimeTypeFormPost                 = "application/x-www-form-urlencoded"
)

var (
	ua = fmt.Sprintf("Go/%s (%s-%s) kubectl-aad-login/msal/%s",
		runtime.Version(),
		runtime.GOARCH,
		runtime.GOOS,
		number,
	)
)

type resourceOwnerToken struct {
	Token       adal.Token       `json:"token"`
	OauthConfig adal.OAuthConfig `json:"oauth"`
	ClientID    string           `json:"clientID"`
	Resource    string           `json:"resource"`
	Username    string           `json:"username"`
	Password    string           `json:"password"`
}

// ResourceOwnerToken encapsulates a Token created for a Resource Owner flow
type ResourceOwnerToken struct {
	inner       resourceOwnerToken
	refreshLock *sync.RWMutex
}

// NewResourceOwnerToken returns a ResourceOwnerToken
func NewResourceOwnerToken(oauthConfig adal.OAuthConfig, clientID, username, password, resource string) (ResourceOwnerToken, error) {
	return ResourceOwnerToken{
		inner: resourceOwnerToken{
			Token:       adal.Token{},
			OauthConfig: oauthConfig,
			ClientID:    clientID,
			Resource:    resource,
			Username:    username,
			Password:    password,
		},
		refreshLock: &sync.RWMutex{},
	}, nil
}

func (spt *ResourceOwnerToken) refreshInternal(ctx context.Context, resource string) error {
	req, err := http.NewRequest(http.MethodPost, spt.inner.OauthConfig.TokenEndpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("Failed to build the refresh request. Error = '%v'", err)
	}
	req.Header.Add("User-Agent", ua)
	v := url.Values{}
	v.Set("client_id", spt.inner.ClientID)
	v.Set("scope", fmt.Sprintf("offline_access %s/.default", spt.inner.Resource))

	if spt.inner.Token.RefreshToken != "" {
		v.Set("grant_type", oAuthGrantTypeRefreshToken)
		v.Set("refresh_token", spt.inner.Token.RefreshToken)
	} else {
		v.Set("grant_type", oAuthGrantTypeResourceOwnerToken)
		v.Set("password", spt.inner.Password)
		v.Set("username", spt.inner.Username)
	}

	s := v.Encode()
	body := ioutil.NopCloser(strings.NewReader(s))
	req.ContentLength = int64(len(s))
	req.Header.Set(contentType, mimeTypeFormPost)
	req.Body = body

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to execute the refresh request. Error = '%v'", err)
	}
	defer resp.Body.Close()
	rb, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		if err != nil {
			return fmt.Errorf("Refresh request failed. Status Code = '%d'. Failed reading response body: %v", resp.StatusCode, err)
		}
		return fmt.Errorf("Refresh request failed. Status Code = '%d'. Response body: %s", resp.StatusCode, string(rb))
	}
	var token adal.Token
	err = json.Unmarshal(rb, &token)
	if err != nil {
		return fmt.Errorf("adal: Failed to unmarshal the service principal token during refresh. Error = '%v' JSON = '%s'", err, string(rb))
	}

	spt.inner.Token = token

	return nil
}

// Refresh obtains a fresh token.
func (spt *ResourceOwnerToken) Refresh() error {
	return spt.RefreshWithContext(context.Background())
}

// RefreshWithContext obtains a fresh token.
func (spt *ResourceOwnerToken) RefreshWithContext(ctx context.Context) error {
	spt.refreshLock.Lock()
	defer spt.refreshLock.Unlock()
	return spt.refreshInternal(ctx, spt.inner.Resource)
}

// Token returns a copy of the current token.
func (spt *ResourceOwnerToken) Token() adal.Token {
	spt.refreshLock.RLock()
	defer spt.refreshLock.RUnlock()
	return spt.inner.Token
}
