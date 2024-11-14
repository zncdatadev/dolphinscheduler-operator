package security

import (
	"fmt"
	"path"
	"strings"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ AuthenticationConfigGenerator = &LDAPAuthenticationConfigGenerator{}

func NewLDAPAuthenticationConfigGenerator(ldap *authv1alpha1.LDAPProvider) *LDAPAuthenticationConfigGenerator {
	return &LDAPAuthenticationConfigGenerator{
		LDAPProvider: *ldap,
	}
}

var _ AuthenticationConfigGenerator = &LDAPAuthenticationConfigGenerator{}

type LDAPAuthenticationConfigGenerator struct {
	authv1alpha1.LDAPProvider
}

// Generate implements AuthenticationConfigGenerator.
func (l *LDAPAuthenticationConfigGenerator) Generate() (map[string]interface{}, error) {

	uidAttr := "uid"
	mailAttr := "mail"
	if l.LDAPFieldNames != nil {
		if l.LDAPFieldNames.Uid != "" {
			uidAttr = l.LDAPFieldNames.Uid
		}
		if l.LDAPFieldNames.Email != "" {
			mailAttr = l.LDAPFieldNames.Email
		}
	}

	ldapUrls := fmt.Sprintf("ldap://%s:%d", l.Hostname, l.Port)
	envKeyPrefix := "SECURITY_AUTHENTICATION_LDAP_"
	evns := map[string]interface{}{
		SecurityAuthenticationType: string(LDAP),
		// envKeyPrefix + "USER_ADMIN":              "read-only-admin", // export in command args
		envKeyPrefix + "URLS":                    ldapUrls,
		envKeyPrefix + "BASE-DN":                 l.SearchBase,
		envKeyPrefix + "USER_IDENTITY-ATTRIBUTE": uidAttr,
		envKeyPrefix + "USER_EMAIL-ATTRIBUTE":    mailAttr,
		envKeyPrefix + "USER_NOT-EXIST-ACTION":   "CREATE",
	}

	if l.TLS != nil {
		// WIP: support TLS
		evns[envKeyPrefix+"SSL_ENABLE"] = "true"
		evns[envKeyPrefix+"SSL_TRUST_STORE"] = path.Join(constants.KubedoopTlsDir, "truststore.p12")
		evns[envKeyPrefix+"SSL_TRUST_STORE_PASSWORD"] = ""
	}
	return evns, nil
}

// export ldap bind user and password by k8s-search
const (
	// security.authentication.type
	SecurityAuthenticationType = "SECURITY_AUTHENTICATION_TYPE"

	LdapBindCredintialsUser = "SECURITY_AUTHENTICATION_LDAP_USERNAME"
	LdapBindCredintialsPass = "SECURITY_AUTHENTICATION_LDAP_PASSWORD"
	LdapUserAdmin           = "SECURITY_AUTHENTICATION_LDAP_USER_ADMIN"

	LdapSecretUserKey = "user"
	LdapSecretPassKey = "password"
)

func ExtractLdapCredintialsAndExportCommand() string {
	// 1. Get ldap credentials from secret mount path
	// 2. Export ldap credentials to env
	userCredentialsSecret := path.Join(constants.KubedoopSecretDir, dolphinv1alpha1.LdapBindCredintialsVolumeName, LdapSecretUserKey)
	passCredentialsSecret := path.Join(constants.KubedoopSecretDir, dolphinv1alpha1.LdapBindCredintialsVolumeName, LdapSecretPassKey)
	cmd := fmt.Sprintf(`export SECURITY_AUTHENTICATION_LDAP_USERNAME="$(cat %s)";
export SECURITY_AUTHENTICATION_LDAP_PASSWORD="$(cat %s)";
export SECURITY_AUTHENTICATION_LDAP_USER_ADMIN="$(cat %s | grep -oP 'uid=\K[^,]+')";
echo "show ldap useranme and admin user: ldap-username: $SECURITY_AUTHENTICATION_LDAP_USERNAME, ldap-user-admin: $SECURITY_AUTHENTICATION_LDAP_USER_ADMIN "`, userCredentialsSecret, passCredentialsSecret, userCredentialsSecret)
	return cmd
}

func AddLdapCredintialsVolumesAndVolumeMounts(bindCredentials commonsv1alpha1.Credentials) (ldapSecretVolume *corev1.Volume, ldapSecretVolumeMount *corev1.VolumeMount) {
	scopes := []string{}
	if bindCredentials.Scope != nil {
		if bindCredentials.Scope.Node {
			scopes = append(scopes, "node")
		}
		if bindCredentials.Scope.Pod {
			scopes = append(scopes, "pod")
		}
		if len(bindCredentials.Scope.Services) > 0 {
			scopes = append(scopes, bindCredentials.Scope.Services...)
		}
	}
	annotations := map[string]string{
		constants.AnnotationSecretsClass: bindCredentials.SecretClass,
	}
	if len(scopes) > 0 {
		annotations[constants.AnnotationSecretsScope] = strings.Join(scopes, constants.CommonDelimiter)
	}
	ldapSecretVolume = &corev1.Volume{
		Name: dolphinv1alpha1.LdapBindCredintialsVolumeName,
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
						StorageClassName: constants.SecretStorageClassPtr(),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Mi"),
							},
						},
					},
				},
			},
		},
	}
	ldapSecretVolumeMount = &corev1.VolumeMount{
		Name:      dolphinv1alpha1.LdapBindCredintialsVolumeName,
		MountPath: path.Join(constants.KubedoopSecretDir, dolphinv1alpha1.LdapBindCredintialsVolumeName),
		ReadOnly:  true,
	}
	return ldapSecretVolume, ldapSecretVolumeMount
}
