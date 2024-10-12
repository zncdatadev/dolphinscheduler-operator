package security

import (
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
)

type AuthenticationType string

const (
	LDAP   AuthenticationType = "LDAP"
	OIDC   AuthenticationType = "OIDC"
	TLS    AuthenticationType = "TLS"
	Static AuthenticationType = "STATIC"
)

var AuthenticationProviderOption func(*AuthenticationProvider)

func NewOidcProvider(
	authType AuthenticationType,
	providerHint OIDCIdentityProvierHit,
	oidcCredentialSecret *dolphinv1alpha1.OidcCredentialSecretSpec,
	provider *authv1alpha1.AuthenticationProvider) *AuthenticationProvider {
	return &AuthenticationProvider{
		AuthType:             authType,
		IdentityProviderHint: providerHint,
		OidcCredentialSecret: oidcCredentialSecret,
		Provider:             provider,
	}
}

func NewLdapProvider(
	authType AuthenticationType,
	provider *authv1alpha1.AuthenticationProvider) *AuthenticationProvider {
	return &AuthenticationProvider{
		AuthType: authType,
		Provider: provider,
	}
}

type AuthenticationProvider struct {
	AuthType AuthenticationType

	IdentityProviderHint OIDCIdentityProvierHit
	OidcCredentialSecret *dolphinv1alpha1.OidcCredentialSecretSpec

	Provider *authv1alpha1.AuthenticationProvider
}

type AuthenticationConfigGenerator interface {
	Generate() (map[string]interface{}, error)
}
