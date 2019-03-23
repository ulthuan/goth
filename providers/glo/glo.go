// Package github implements the OAuth2 protocol for authenticating users through Github.
// This package can be used as a reference implementation of an OAuth2 provider for Goth.
package glo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

// These vars define the Authentication, Token, and API URLS for GitHub. If
// using GitHub enterprise you should change these values before calling New.
//
// Examples:
//	github.AuthURL = "https://github.acme.com/login/oauth/authorize
//	github.TokenURL = "https://github.acme.com/login/oauth/access_token
//	github.ProfileURL = "https://github.acme.com/api/v3/user
//	github.EmailURL = "https://github.acme.com/api/v3/user/emails
var (
	AuthURL    = "https://app.gitkraken.com/oauth/authorize"
	TokenURL   = "https://api.gitkraken.com/oauth/access_token"
	ProfileURL = "https://gloapi.gitkraken.com/v1/glo/user"
)

// New creates a new Github provider, and sets up important connection details.
// You should always call `github.New` to get a new Provider. Never try to create
// one manually.
func New(clientKey, secret, callbackURL string, scopes ...string) *Provider {
	return NewCustomisedURL(clientKey, secret, callbackURL, AuthURL, TokenURL, ProfileURL, scopes...)
}

// NewCustomisedURL is similar to New(...) but can be used to set custom URLs to connect to
func NewCustomisedURL(clientKey, secret, callbackURL, authURL, tokenURL, profileURL, scopes ...string) *Provider {
	p := &Provider{
		ClientKey:    clientKey,
		Secret:       secret,
		CallbackURL:  callbackURL,
		providerName: "glo",
		profileURL:   profileURL,
	}
	p.config = newConfig(p, authURL, tokenURL, scopes)
	return p
}

// Provider is the implementation of `goth.Provider` for accessing Github.
type Provider struct {
	ClientKey    string
	Secret       string
	CallbackURL  string
	HTTPClient   *http.Client
	config       *oauth2.Config
	providerName string
	profileURL   string
	emailURL     string
}

// Name is the name used to retrieve this provider later.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is a no-op for the github package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth asks Github for an authentication end-point.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	url := p.config.AuthCodeURL(state)
	session := &Session{
		AuthURL: url,
	}
	return session, nil
}

// FetchUser will go to Github and access basic information about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	sess := session.(*Session)
	user := goth.User{
		AccessToken: sess.AccessToken,
		Provider:    p.Name(),
	}

	if user.AccessToken == "" {
		// data is not yet retrieved since accessToken is still empty
		return user, fmt.Errorf("%s cannot get user information without accessToken", p.providerName)
	}

	response, err := p.Client().Get(p.profileURL + "?access_token=" + url.QueryEscape(sess.AccessToken))
	if err != nil {
		return user, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return user, fmt.Errorf("GitHub API responded with a %d trying to fetch user information", response.StatusCode)
	}

	bits, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return user, err
	}

	err = json.NewDecoder(bytes.NewReader(bits)).Decode(&user.RawData)
	if err != nil {
		return user, err
	}

	err = userFromReader(bytes.NewReader(bits), &user)
	if err != nil {
		return user, err
	}

	return user, err
}

func userFromReader(reader io.Reader, user *goth.User) error {
	u := struct {
		ID       int    `json:"id"`
		Email    string `json:"email"`
		Bio      string `json:"bio"`
		Name     string `json:"name"`
		Login    string `json:"login"`
		Picture  string `json:"avatar_url"`
		Location string `json:"location"`
	}{}

	err := json.NewDecoder(reader).Decode(&u)
	if err != nil {
		return err
	}

	user.Name = u.Name
	user.NickName = u.Login
	user.Email = u.Email
	user.Description = u.Bio
	user.AvatarURL = u.Picture
	user.UserID = strconv.Itoa(u.ID)
	user.Location = u.Location

	return err
}

func newConfig(provider *Provider, authURL, tokenURL string, scopes []string) *oauth2.Config {
	c := &oauth2.Config{
		ClientID:     provider.ClientKey,
		ClientSecret: provider.Secret,
		RedirectURL:  provider.CallbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes: []string{},
	}

	for _, scope := range scopes {
		c.Scopes = append(c.Scopes, scope)
	}

	return c
}

//RefreshToken refresh token is not provided by github
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	return nil, errors.New("Refresh token is not provided by github")
}

//RefreshTokenAvailable refresh token is not provided by github
func (p *Provider) RefreshTokenAvailable() bool {
	return false
}
