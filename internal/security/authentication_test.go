/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package security

import (
	"context"
	"strings"
	"testing"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testOIDCHostname               = "keycloak.example.com"
	testMissingAuthenticationClass = "missing"
	testAuthenticationNamespace    = "default"
	testOIDCCredentialSecret       = "oidc-credentials"
)

func validOIDCCredentialSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testOIDCCredentialSecret, Namespace: testAuthenticationNamespace},
		Data: map[string][]byte{
			OidcClientIdKey: []byte("dolphinscheduler-client"),
			OidcSecretKey:   []byte("do-not-render-this-secret"),
		},
	}
}

func TestAuthenticationReturnsResolutionErrors(t *testing.T) {
	oidcClass := &authv1alpha1.AuthenticationClass{
		ObjectMeta: metav1.ObjectMeta{Name: oidcGenericProviderHint},
		Spec: authv1alpha1.AuthenticationClassSpec{
			AuthenticationProvider: &authv1alpha1.AuthenticationProvider{
				OIDC: &authv1alpha1.OIDCProvider{
					Hostname:       testOIDCHostname,
					Port:           443,
					PrincipalClaim: "preferred_username",
					ProviderHint:   string(Keycloak),
				},
			},
		},
	}

	tests := []struct {
		name     string
		objects  []ctrlclient.Object
		authSpec []dolphinv1alpha1.AuthenticationSpec
		wantErr  string
	}{
		{
			name: "missing AuthenticationClass",
			authSpec: []dolphinv1alpha1.AuthenticationSpec{
				{AuthenticationClass: testMissingAuthenticationClass},
			},
			wantErr: authenticationClassResolutionError,
		},
		{
			name:    "missing AuthenticationClass before a valid provider",
			objects: []ctrlclient.Object{oidcClass.DeepCopy(), validOIDCCredentialSecret()},
			authSpec: []dolphinv1alpha1.AuthenticationSpec{
				{AuthenticationClass: testMissingAuthenticationClass},
				{
					AuthenticationClass: oidcGenericProviderHint,
					Oidc: &dolphinv1alpha1.OidcCredentialSecretSpec{
						Secret: testOIDCCredentialSecret,
					},
				},
			},
			wantErr: authenticationClassResolutionError,
		},
		{
			name:    "missing AuthenticationClass after a valid provider",
			objects: []ctrlclient.Object{oidcClass.DeepCopy(), validOIDCCredentialSecret()},
			authSpec: []dolphinv1alpha1.AuthenticationSpec{
				{
					AuthenticationClass: oidcGenericProviderHint,
					Oidc: &dolphinv1alpha1.OidcCredentialSecretSpec{
						Secret: testOIDCCredentialSecret,
					},
				},
				{AuthenticationClass: testMissingAuthenticationClass},
			},
			wantErr: authenticationClassResolutionError,
		},
		{
			name:    "OIDC credentials omitted",
			objects: []ctrlclient.Object{oidcClass.DeepCopy()},
			authSpec: []dolphinv1alpha1.AuthenticationSpec{
				{AuthenticationClass: oidcGenericProviderHint},
			},
			wantErr: "OIDC credential secret is required",
		},
		{
			name:    "OIDC credential secret name blank",
			objects: []ctrlclient.Object{oidcClass.DeepCopy()},
			authSpec: []dolphinv1alpha1.AuthenticationSpec{
				{
					AuthenticationClass: oidcGenericProviderHint,
					Oidc:                &dolphinv1alpha1.OidcCredentialSecretSpec{Secret: "  "},
				},
			},
			wantErr: "OIDC credential secret name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := authv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatalf("add authentication API to scheme: %v", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("add core API to scheme: %v", err)
			}
			client := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

			result, err := Authentication(context.Background(), client, testAuthenticationNamespace, tt.authSpec)
			if err == nil {
				t.Fatalf("Authentication() error = nil, want an error containing %q", tt.wantErr)
			}
			if result != nil {
				t.Fatalf("Authentication() result = %#v, want nil", result)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Authentication() error = %q, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestAuthenticationValidatesOIDCCredentialSecret(t *testing.T) {
	authClass := &authv1alpha1.AuthenticationClass{
		ObjectMeta: metav1.ObjectMeta{Name: oidcGenericProviderHint},
		Spec: authv1alpha1.AuthenticationClassSpec{AuthenticationProvider: &authv1alpha1.AuthenticationProvider{
			OIDC: &authv1alpha1.OIDCProvider{
				Hostname: testOIDCHostname, PrincipalClaim: "preferred_username", ProviderHint: string(Keycloak),
			},
		}},
	}
	authSpec := []dolphinv1alpha1.AuthenticationSpec{{
		AuthenticationClass: oidcGenericProviderHint,
		Oidc:                &dolphinv1alpha1.OidcCredentialSecretSpec{Secret: testOIDCCredentialSecret},
	}}

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr string
	}{
		{name: "missing Secret", wantErr: "failed to get OIDC credential Secret"},
		{
			name: "missing client id",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testOIDCCredentialSecret, Namespace: testAuthenticationNamespace},
				Data:       map[string][]byte{OidcSecretKey: []byte("secret")},
			},
			wantErr: "OIDC credential Secret key is missing",
		},
		{
			name: "empty client secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testOIDCCredentialSecret, Namespace: testAuthenticationNamespace},
				Data: map[string][]byte{
					OidcClientIdKey: []byte("client"),
					OidcSecretKey:   {},
				},
			},
			wantErr: "OIDC credential Secret key is empty",
		},
		{name: "both keys are non-empty", secret: validOIDCCredentialSecret()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []ctrlclient.Object{authClass.DeepCopy()}
			if tt.secret != nil {
				objects = append(objects, tt.secret.DeepCopy())
			}
			scheme := runtime.NewScheme()
			if err := authv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatalf("add authentication API to scheme: %v", err)
			}
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatalf("add core API to scheme: %v", err)
			}
			client := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			result, err := Authentication(context.Background(), client, testAuthenticationNamespace, authSpec)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Authentication() unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Authentication() result = nil, want resolved configuration")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Authentication() error = %v, want substring %q", err, tt.wantErr)
			}
			if result != nil {
				t.Fatalf("Authentication() result = %#v, want nil", result)
			}
		})
	}
}
