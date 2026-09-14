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
	"strings"
	"testing"

	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
)

func TestGetIssuer(t *testing.T) {
	tests := []struct {
		name     string
		provider *authv1alpha1.OIDCProvider
		want     string
	}{
		{
			name: "omits an unset port and normalizes the root path",
			provider: &authv1alpha1.OIDCProvider{
				Hostname: testOIDCHostname,
				RootPath: "/",
			},
			want: "http://" + testOIDCHostname,
		},
		{
			name: "omits the HTTPS default port and trailing slash",
			provider: &authv1alpha1.OIDCProvider{
				Hostname: testOIDCHostname,
				Port:     443,
				RootPath: "/realms/kubedoop/",
				TLS:      &authv1alpha1.OIDCTls{},
			},
			want: "https://" + testOIDCHostname + "/realms/kubedoop",
		},
		{
			name: "keeps a non-default port",
			provider: &authv1alpha1.OIDCProvider{
				Hostname: testOIDCHostname,
				Port:     8080,
				RootPath: "/realms/kubedoop",
			},
			want: "http://" + testOIDCHostname + ":8080/realms/kubedoop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getIssuer(tt.provider); got != tt.want {
				t.Fatalf("getIssuer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateOIDCTLS(t *testing.T) {
	tests := []struct {
		name         string
		verification *commonsv1alpha1.TLSVerificationSpec
		wantErr      string
	}{
		{name: "default JVM trust store"},
		{
			name: "explicit Web PKI",
			verification: &commonsv1alpha1.TLSVerificationSpec{Server: &commonsv1alpha1.ServerVerification{
				CACert: &commonsv1alpha1.CACert{WebPki: &commonsv1alpha1.WebPki{}},
			}},
		},
		{
			name:         "verification none",
			verification: &commonsv1alpha1.TLSVerificationSpec{None: &commonsv1alpha1.NoneVerification{}},
			wantErr:      "OIDC TLS verification mode none is not supported",
		},
		{
			name: "custom CA SecretClass",
			verification: &commonsv1alpha1.TLSVerificationSpec{Server: &commonsv1alpha1.ServerVerification{
				CACert: &commonsv1alpha1.CACert{SecretClass: "company-ca"},
			}},
			wantErr: "OIDC TLS custom CA SecretClass is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &authv1alpha1.OIDCProvider{TLS: &authv1alpha1.OIDCTls{Verification: tt.verification}}
			err := validateOIDCTLS(provider)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateOIDCTLS() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateOIDCTLS() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
