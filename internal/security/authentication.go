package security

import (
	"context"
	"maps"
	"slices"
	"strings"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	opgosecurity "github.com/zncdatadev/operator-go/pkg/security"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DEFAULT_OIDC_PROVIDER is the assumed OIDC provider if no hint is given in the AuthClass
	DEFAULT_OIDC_PROVIDER              OIDCIdentityProvierHit = Keycloak
	oidcGenericProviderHint                                   = "oidc"
	authenticationClassResolutionError                        = "failed to get AuthenticationClass"
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

// AuthenticationResult is the resolved authentication contribution to the api-server container.
type AuthenticationResult struct {
	// EnvVars are the security env vars, sorted by name (the legacy SortedMap rendering).
	EnvVars []corev1.EnvVar

	// LdapExportCommand, when non-empty, is the script prologue exporting the LDAP bind
	// credentials from the mounted secret files.
	LdapExportCommand string

	// Provisioner supplies the LDAP bind-credentials CSI volume + mount; nil without LDAP.
	Provisioner *opgosecurity.SecretProvisioner
}

// Authentication generates the authentication configuration for the Scheduler.
// It resolves the AuthenticationClass and based on the provider in the
// AuthenticationClass, it generates the configuration for the Scheduler.
// Supported providers are LDAP and OIDC.
func Authentication(
	ctx context.Context,
	c ctrlclient.Client,
	namespace string,
	authSpec []dolphinv1alpha1.AuthenticationSpec,
) (*AuthenticationResult, error) {
	providers, err := resolveAuthentications(ctx, c, namespace, authSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve authentication providers")
	}

	config, err := createAuthenticationConfig(providers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create authentication config")
	}

	result := &AuthenticationResult{
		EnvVars: sortedEnvVars(config),
	}

	for _, provider := range providers {
		if provider.AuthType == LDAP {
			if provider.Provider.LDAP == nil || provider.Provider.LDAP.BindCredentials == nil {
				return nil, errors.New("ldap provider or bind credentials cannot be nil")
			}
			result.Provisioner = LdapBindCredentialsProvisioner(*provider.Provider.LDAP.BindCredentials)
			result.LdapExportCommand = ExtractLdapCredintialsAndExportCommand()
			break
		}
	}

	return result, nil
}

// sortedEnvVars converts the generated security config (string values, or corev1.EnvVarSource
// for secret-backed values) into an env var list sorted by name — the legacy SortedMap order.
func sortedEnvVars(config map[string]interface{}) []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0, len(config))
	for _, name := range slices.Sorted(maps.Keys(config)) {
		switch value := config[name].(type) {
		case string:
			envs = append(envs, corev1.EnvVar{Name: name, Value: value})
		case corev1.EnvVarSource:
			source := value
			envs = append(envs, corev1.EnvVar{Name: name, ValueFrom: &source})
		}
	}
	return envs
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
			return config, err
		}
		var providerHintSecurityConfig map[string]interface{}
		providerHintSecurityConfig, err = authenticationConfigGenerator.Generate()
		if err != nil {
			return config, err
		}
		maps.Copy(config, providerHintSecurityConfig)
	}
	return config, err
}

func resolveAuthentications(
	ctx context.Context,
	c ctrlclient.Client,
	namespace string,
	authentications []dolphinv1alpha1.AuthenticationSpec,
) ([]AuthenticationProvider, error) {
	providers := make([]AuthenticationProvider, 0, len(authentications))
	for _, dolphinAuth := range authentications {
		authClass, err := resolveAuthenticationClass(ctx, c, dolphinAuth.AuthenticationClass)
		if err != nil {
			return nil, err
		}

		authProvider, err := getAuthenticationProvider(authClass, dolphinAuth.Oidc)
		if err != nil {
			return nil, err
		}
		if !isAuthenticationSupported(authProvider.AuthType) {
			return nil, errors.NewWithDetails(
				"auth type is not supported", "actual type", authProvider.AuthType)
		}
		if authProvider.AuthType == OIDC {
			if err := validateOIDCCredentialSecret(ctx, c, namespace, dolphinAuth.Oidc); err != nil {
				return nil, err
			}
		}
		providers = append(providers, *authProvider)
	}
	return providers, nil
}

func validateOIDCCredentialSecret(
	ctx context.Context,
	c ctrlclient.Client,
	namespace string,
	credentialSpec *dolphinv1alpha1.OidcCredentialSecretSpec,
) error {
	if credentialSpec == nil {
		return errors.New("OIDC credential secret is required")
	}
	secretName := strings.TrimSpace(credentialSpec.Secret)
	if secretName == "" {
		return errors.New("OIDC credential secret name is required")
	}

	secret := &corev1.Secret{}
	objectKey := types.NamespacedName{Namespace: namespace, Name: secretName}
	if err := c.Get(ctx, objectKey, secret); err != nil {
		return errors.WrapWithDetails(err, "failed to get OIDC credential Secret", "secret", objectKey.String())
	}
	for _, key := range []string{OidcClientIdKey, OidcSecretKey} {
		value, found := secret.Data[key]
		if !found {
			return errors.NewWithDetails("OIDC credential Secret key is missing", "secret", objectKey.String(), "key", key)
		}
		if len(value) == 0 {
			return errors.NewWithDetails("OIDC credential Secret key is empty", "secret", objectKey.String(), "key", key)
		}
	}
	return nil
}

func resolveAuthenticationClass(
	ctx context.Context,
	c ctrlclient.Client,
	authClassRef string,
) (*authv1alpha1.AuthenticationClass, error) {
	// AuthenticationClass is cluster-scoped: no namespace in the lookup key.
	authClassObject := &authv1alpha1.AuthenticationClass{}
	if err := c.Get(ctx, types.NamespacedName{Name: authClassRef}, authClassObject); err != nil {
		return nil, errors.WrapWithDetails(err, authenticationClassResolutionError, "authentication_class", authClassRef)
	}
	return authClassObject, nil
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
	switch {
	case provider.OIDC != nil:
		if err = validateOIDCTLS(provider.OIDC); err != nil {
			return
		}
		var providerHint OIDCIdentityProvierHit
		providerHint, err = getOidcProviderHint(provider.OIDC)
		if err != nil {
			return
		}
		authProvider = NewOidcProvider(OIDC, providerHint, oidcSecretSpec, provider)
	case provider.TLS != nil:
		err = errors.New("TLS authentication provider is not supported")
	case provider.Static != nil:
		err = errors.New("static authentication provider is not supported")
	case provider.LDAP != nil:
		authProvider = NewLdapProvider(LDAP, provider)
	default:
		err = errors.New("no supported authentication provider is configured")
	}
	return
}

func getOidcProviderHint(oidcProvider *authv1alpha1.OIDCProvider) (hint OIDCIdentityProvierHit, err error) {
	switch oidcProvider.ProviderHint {
	case string(Keycloak):
		hint = Keycloak
	case "github":
		hint = Github
	case oidcGenericProviderHint:
		hint = Github // todo:for test only
	default:
		err = errors.NewWithDetails("oidc provider hint is not supported", "actual provider hint", oidcProvider.ProviderHint)
	}
	return
}

func isAuthenticationSupported(authType AuthenticationType) bool {
	return slices.Contains(SUPPORTED_AUTHENTICATION_CLASS_PROVIDERS, authType)
}
