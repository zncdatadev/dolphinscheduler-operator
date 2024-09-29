package security

import (
	"fmt"
	"strconv"
	"strings"

	"emperror.dev/errors"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// OIDCIdentityProvierHit is a string that indicates the identity provider.
type OIDCIdentityProvierHit string

const (
	Keycloak OIDCIdentityProvierHit = "keycloak"
	Github   OIDCIdentityProvierHit = "github"
)

var _ AuthenticationConfigGenerator = &OidcAuthenticationConfigGenerator{}

func NewOidcAuthenticationConfigGenerator(oidcProvider *AuthenticationProvider) *OidcAuthenticationConfigGenerator {
	return &OidcAuthenticationConfigGenerator{
		AuthenticationProvider: *oidcProvider,
	}
}

var _ AuthenticationConfigGenerator = &OidcAuthenticationConfigGenerator{}

type OidcAuthenticationConfigGenerator struct {
	AuthenticationProvider
}

// Generate implements euthenticationConfigGenerator.
func (o *OidcAuthenticationConfigGenerator) Generate() (map[string]interface{}, error) {
	if o.AuthType != OIDC {
		return nil, errors.NewWithDetails("authentication type is not OIDC", "actual type", o.AuthType)
	}
	var oidcMetadataExtractor OidcProviderMetadataExtractor
	switch providerHint := o.IdentityProviderHint; providerHint {
	case Keycloak:
		oidcMetadataExtractor = NewKeycloakMetadata(o.Provider)
	case Github:
		oidcMetadataExtractor = NewGithubMetadata(o.Provider)
	default:
		return nil, errors.NewWithDetails("oidc provider hint is not supported", "provider hint", providerHint)
	}

	// oidc client credentials env from secret
	var secretToEnvFunc = func(secretKey string) corev1.EnvVarSource {
		return corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: o.OidcCredentialSecret.Secret,
				},
				Key: secretKey,
			},
		}
	}

	providerHint := oidcMetadataExtractor.ProviderHint()
	return map[string]interface{}{
		"SECURITY_AUTHENTICATION_OAUTH2_ENABLE":                                 "true",
		CreateOidcProviderHintSecurityEnvKey(providerHint, "AUTHORIZATION-URI"): oidcMetadataExtractor.AuthorizationUri(),
		CreateOidcProviderHintSecurityEnvKey(providerHint, "REDIRECT-URI"):      RedirectUri,
		CreateOidcProviderHintSecurityEnvKey(providerHint, "CLIENT-ID"):         secretToEnvFunc(OidcClientIdKey),
		CreateOidcProviderHintSecurityEnvKey(providerHint, "CLIENT-SECRET"):     secretToEnvFunc(OidcSecretKey),
		CreateOidcProviderHintSecurityEnvKey(providerHint, "TOKEN-URI"):         oidcMetadataExtractor.TokenUri(),
		CreateOidcProviderHintSecurityEnvKey(providerHint, "USER-INFO-URI"):     oidcMetadataExtractor.UserInfoUri(),
		CreateOidcProviderHintSecurityEnvKey(providerHint, "CALLBACK-URL"):      CallbackUri,
		CreateOidcProviderHintSecurityEnvKey(providerHint, "ICON-URI"):          "",
		CreateOidcProviderHintSecurityEnvKey(providerHint, "PROVIDER"):          string(oidcMetadataExtractor.ProviderHint()),
	}, nil
}

type OidcProviderMetadataExtractor interface {
	AuthorizationUri() string
	TokenUri() string
	UserInfoUri() string
	ProviderHint() OIDCIdentityProvierHit
}

// -------------------------------keycloak metadata-----------------------------
var _ OidcProviderMetadataExtractor = &KeycloakMetadata{}

func NewKeycloakMetadata(providerSpec *authv1alpha1.AuthenticationProvider) *KeycloakMetadata {
	issuer := getIssuer(providerSpec.OIDC)
	return &KeycloakMetadata{
		AuthenticationProvider: providerSpec,
		issuer:                 issuer,
	}
}

type KeycloakMetadata struct {
	*authv1alpha1.AuthenticationProvider

	issuer string
}

// ProviderHint implements OidcProviderMetadataExtractor.
func (k *KeycloakMetadata) ProviderHint() OIDCIdentityProvierHit {
	return Keycloak
}

// AuthorizationUri implements OidcProviderMetadataExtractor.
func (k *KeycloakMetadata) AuthorizationUri() string {
	return k.issuer + "/protocol/openid-connect/auth"
}

// TokenUri implements OidcProviderMetadataExtractor.
func (k *KeycloakMetadata) TokenUri() string {
	return k.issuer + "/protocol/openid-connect/token"
}

// UserInfoUri implements OidcProviderMetadataExtractor.
func (k *KeycloakMetadata) UserInfoUri() string {
	return k.issuer + "/protocol/openid-connect/userinfo"
}

// issuer
func getIssuer(oidcProvider *authv1alpha1.OIDCProvider) string {
	schme := GetScheme(oidcProvider)
	host := oidcProvider.Hostname
	port := oidcProvider.Port
	rootPath := oidcProvider.RootPath

	return fmt.Sprintf("%s://%s:%s%s", schme, host, strconv.Itoa(port), rootPath)
}

// get schema
func GetScheme(oidcProvider *authv1alpha1.OIDCProvider) string {
	if IsTls(oidcProvider) {
		return "https"
	}
	return "http"
}

// is oidc provider enabled tls
func IsTls(oidcProvider *authv1alpha1.OIDCProvider) bool {
	return oidcProvider.TLS != nil
}

// ----------------------------------github metadata-----------------------------
var _ OidcProviderMetadataExtractor = &GithubMetadata{}

func NewGithubMetadata(providerSpec *authv1alpha1.AuthenticationProvider) *GithubMetadata {
	return &GithubMetadata{
		AuthenticationProvider: providerSpec,
	}
}

type GithubMetadata struct {
	*authv1alpha1.AuthenticationProvider
}

// ProviderHint implements OidcProviderMetadataExtractor.
func (g *GithubMetadata) ProviderHint() OIDCIdentityProvierHit {
	return Github
}

func (g *GithubMetadata) AuthorizationUri() string {
	return "https://github.com/login/oauth/authorize"
}

func (g *GithubMetadata) TokenUri() string {
	return "https://github.com/login/oauth/access_token"
}

func (g *GithubMetadata) UserInfoUri() string {
	return "https://api.github.com/user"
}

func CreateOidcProviderHintSecurityEnvKey(proivderHint OIDCIdentityProvierHit, metadataItem string) string {
	envPrefix := fmt.Sprintf("SECURITY_AUTHENTICATION_OAUTH2_PROVIDER_%s_", strings.ToUpper(string(proivderHint)))
	return fmt.Sprintf("%s%s", envPrefix, metadataItem)
}
