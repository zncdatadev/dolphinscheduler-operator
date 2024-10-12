package security

import (
	"context"
	"maps"
	"slices"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var authenticationLogger = ctrl.Log.WithName("authentication-log")

const (
	// DEFAULT_OIDC_PROVIDER is the assumed OIDC provider if no hint is given in the AuthClass
	DEFAULT_OIDC_PROVIDER OIDCIdentityProvierHit = Keycloak
)

const (
	RedirectUri = "http://127.0.0.1:12345/dolphinscheduler/redirect/login/oauth2"
	CallbackUri = "http://127.0.0.1:12345/dolphinscheduler/ui/login"

	OidcClientIdKey = "CLIENT_ID"
	OidcSecretKey   = "CLIENT_SECRET"
)

var (
	SUPPORTED_AUTHENTICATION_CLASS_PROVIDERS = []AuthenticationType{LDAP, OIDC}
	SUPPORTED_OIDC_PROVIDERS                 = []OIDCIdentityProvierHit{Github}
)

type AuthenticationResult struct {
	// dolphin scheduler security configuration
	// this will override the default security configuration in application.yaml
	Config map[string]interface{}

	CredintialsSecrets []string

	// ldap bind credentials secret volume
	LdapVolume *corev1.Volume

	// ldap bind credentials secret volume mount
	LdapVolumeMount *corev1.VolumeMount

	//ldap bind credentials secret name
	LdapBindCredintialsName string
}

// Authentication generates the authentication configuration for the Scheduler.
// It resolves the AuthenticationClass and based on the provider in the
// AuthenticationClass, it generates the configuration for the Scheduler.
// Supported providers are LDAP and OIDC.
// For OIDC, only Keycloak is supported.
func Authentication(
	ctx context.Context,
	client *client.Client,
	authSpec []dolphinv1alpha1.AuthenticationSpec) (result AuthenticationResult, err error) {
	providers, err := resolveAuthentications(ctx, client, authSpec)
	if err != nil {
		authenticationLogger.Error(err, "Failed to resolve AuthenticationClass")
	}
	// security env config
	// this will override the default security configuration in application.yaml
	var config map[string]interface{}
	config, err = createAuthenticationConfig(providers)
	if err != nil {
		authenticationLogger.Error(err, "Failed to create AuthenticationConfig")
		return
	}

	// ldap volume and volume mount
	var volume *corev1.Volume
	var volumeMount *corev1.VolumeMount
	var ldapBindCredentialsName string
	for _, provider := range providers {
		if provider.AuthType == LDAP {
			if provider.Provider.LDAP == nil || provider.Provider.LDAP.BindCredentials == nil {
				err = errors.New("ldap provider or bind credentials cannot be nil")
				return
			}
			volume, volumeMount = AddLdapCredintialsVolumesAndVolumeMounts(*provider.Provider.LDAP.BindCredentials)
			ldapBindCredentialsName = provider.Provider.LDAP.BindCredentials.SecretClass
			break
		}
	}

	// secret names for authentication
	var secretNames = make([]string, 0)
	// oidc secret, we can add other secret in the future
	// this is deprecated, we use env source instead, as oidc clientId and secret key are the same name all,so we can use env source to map it
	// reserve the field for future use
	// for _, provider := range providers {
	// 	secretNames = append(secretNames, provider.OidcCredentialSecret.Secret)
	// }

	return AuthenticationResult{
		Config:                  config,
		LdapVolume:              volume,
		LdapVolumeMount:         volumeMount,
		LdapBindCredintialsName: ldapBindCredentialsName,
		CredintialsSecrets:      secretNames,
	}, nil
}

func createAuthenticationConfig(providers []AuthenticationProvider) (config map[string]interface{}, err error) {
	var authenticationConfigGenerator AuthenticationConfigGenerator
	config = make(map[string]interface{})
	ldapExists := false // ldap resolve the first allways
	for _, provider := range providers {
		authType := provider.AuthType
		switch authType {
		case OIDC:
			authenticationConfigGenerator = NewOidcAuthenticationConfigGenerator(&provider)
		case LDAP:
			if !ldapExists {
				authenticationConfigGenerator = NewLDAPAuthenticationConfigGenerator(provider.Provider.LDAP)
				ldapExists = true
			} else {
				continue
			}
		default:
			err = errors.NewWithDetails("auth type is not supported", "authentication type", authType)
			return
		}
		var providerHintSecurityConfig map[string]interface{}
		providerHintSecurityConfig, err = authenticationConfigGenerator.Generate()
		if err != nil {
			return
		}
		maps.Copy(config, providerHintSecurityConfig)
	}
	return
}

func resolveAuthentications(
	ctx context.Context,
	client *client.Client,
	anthenticantions []dolphinv1alpha1.AuthenticationSpec) (providers []AuthenticationProvider, err error) {
	for _, dolphinAuth := range anthenticantions {
		var authclass *authv1alpha1.AuthenticationClass
		if authclass, err = resolveAuthenticationClass(ctx, client, dolphinAuth.AuthenticationClass); err == nil {
			var authprovider *AuthenticationProvider
			authprovider, err = getAuthenticationProvider(authclass, dolphinAuth.Oidc)
			authType := authprovider.AuthType
			if isAuthenticationSupported(authType) {
				providers = append(providers, *authprovider)
			} else {
				err = errors.NewWithDetails("auth type is not supported", "actual type", authType)
				return
			}
		}
	}
	return
}

func resolveAuthenticationClass(
	ctx context.Context,
	client *client.Client,
	authClassRef string) (authclass *authv1alpha1.AuthenticationClass, err error) {
	authClassObject := &authv1alpha1.AuthenticationClass{}
	if err = client.GetWithOwnerNamespace(ctx, authClassRef, authClassObject); err != nil {
		authenticationLogger.Error(err, "Failed to get AuthenticationClass", "authClass ref", authClassRef, "namespace", client.GetOwnerNamespace())
		return
	}
	authclass = authClassObject
	return
}

func getAuthenticationProvider(
	authClass *authv1alpha1.AuthenticationClass,
	oidcSecretSpec *dolphinv1alpha1.OidcCredentialSecretSpec) (authProvider *AuthenticationProvider, err error) {
	if authClass == nil {
		err = errors.New("AuthenticationClass cannot be nil")
		return
	}
	provider := authClass.Spec.AuthenticationProvider
	if provider == nil {
		err = errors.New("AuthenticationProvider cannot be nil")
		return
	}
	if provider.OIDC != nil {
		var providerHint OIDCIdentityProvierHit
		providerHint, err = getOidcProviderHint(provider.OIDC)
		if err != nil {
			return
		}
		authProvider = NewOidcProvider(OIDC, providerHint, oidcSecretSpec, provider)
	} else if provider.TLS != nil {
		panic("unimplemented")
	} else if provider.Static != nil {
		panic("unimplemented")
	} else if provider.LDAP != nil {
		authProvider = NewLdapProvider(LDAP, provider)
	}
	return
}

func getOidcProviderHint(oidcProvider *authv1alpha1.OIDCProvider) (hint OIDCIdentityProvierHit, err error) {
	switch oidcProvider.Provisioner {
	case "keycloak":
		hint = Keycloak
	case "github":
		hint = Github
	case "oidc":
		hint = Github // todo:for test only
	default:
		err = errors.NewWithDetails("oidc provider hint is not supported", "actual provider hint", oidcProvider.Provisioner)
	}
	return
}

func isAuthenticationSupported(authType AuthenticationType) bool {
	return slices.Contains(SUPPORTED_AUTHENTICATION_CLASS_PROVIDERS, AuthenticationType(authType))
}
