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
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
)

const (
	testLDAPBindDN         = "cn=admin,dc=example,dc=org"
	testLDAPSecretSentinel = "do-not-leak"
)

func TestLDAPAuthenticationConfig(t *testing.T) {
	t.Run("defaults the plain LDAP port", func(t *testing.T) {
		config, err := NewLDAPAuthenticationConfigGenerator(&authv1alpha1.LDAPProvider{
			Hostname: "ldap.example.com",
		}).Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if got, want := config["SECURITY_AUTHENTICATION_LDAP_URLS"], "ldap://ldap.example.com:389"; got != want {
			t.Fatalf("LDAP URL = %q, want %q", got, want)
		}
	})

	t.Run("rejects TLS until trust material is provisioned", func(t *testing.T) {
		_, err := NewLDAPAuthenticationConfigGenerator(&authv1alpha1.LDAPProvider{
			Hostname: "ldap.example.com",
			TLS:      &authv1alpha1.LDAPTLS{},
		}).Generate()
		if err == nil {
			t.Fatal("Generate() error = nil, want LDAP TLS rejection")
		}
	})
}

func TestLdapCredentialsExportCommandRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name      string
		writeUser bool
		user      string
		writePass bool
		password  string
	}{
		{name: "missing username", writePass: true, password: testLDAPSecretSentinel},
		{name: "empty username", writeUser: true, writePass: true, password: testLDAPSecretSentinel},
		{name: "missing password", writeUser: true, user: testLDAPBindDN},
		{name: "empty password", writeUser: true, user: testLDAPBindDN, writePass: true},
		{name: "username without RDN", writeUser: true, user: "invalid", writePass: true, password: testLDAPSecretSentinel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsDir := t.TempDir()
			if tt.writeUser {
				if err := os.WriteFile(filepath.Join(credentialsDir, LdapSecretUserKey), []byte(tt.user), 0o600); err != nil {
					t.Fatalf("write username fixture: %v", err)
				}
			}
			if tt.writePass {
				if err := os.WriteFile(filepath.Join(credentialsDir, LdapSecretPassKey), []byte(tt.password), 0o600); err != nil {
					t.Fatalf("write password fixture: %v", err)
				}
			}

			// #nosec G204 -- this executes the generated fixed script against fake credentials only.
			cmd := exec.Command("/bin/bash", "-euo", "pipefail", "-c",
				ldapCredentialsExportCommand(credentialsDir))
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("LDAP export script succeeded for %s", tt.name)
			}
			if strings.Contains(string(output), testLDAPSecretSentinel) {
				t.Fatalf("LDAP export script leaked a credential: %q", output)
			}
		})
	}
}

func TestLdapCredentialsExportCommand(t *testing.T) {
	tests := []struct {
		name      string
		bindDN    string
		wantAdmin string
	}{
		{name: "common name RDN", bindDN: testLDAPBindDN, wantAdmin: "admin"},
		{name: "user ID RDN", bindDN: "uid=reader,ou=people,dc=example,dc=org", wantAdmin: "reader"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(credentialsDir, LdapSecretUserKey), []byte(tt.bindDN), 0o600); err != nil {
				t.Fatalf("write bind DN fixture: %v", err)
			}
			if err := os.WriteFile(filepath.Join(credentialsDir, LdapSecretPassKey), []byte("test-password"), 0o600); err != nil {
				t.Fatalf("write password fixture: %v", err)
			}

			script := ldapCredentialsExportCommand(credentialsDir) +
				"\nprintf '%s|%s|%s' \"$SECURITY_AUTHENTICATION_LDAP_USERNAME\" \"$SECURITY_AUTHENTICATION_LDAP_PASSWORD\" \"$SECURITY_AUTHENTICATION_LDAP_USER_ADMIN\""
			// #nosec G204 -- this executes the generated fixed script against fake credentials only.
			cmd := exec.Command("/bin/bash", "-euo", "pipefail", "-c", script)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatalf("execute LDAP export script: %v (stderr: %q)", err, stderr.String())
			}
			want := tt.bindDN + "|test-password|" + tt.wantAdmin
			if string(stdout) != want {
				t.Fatalf("LDAP export output = %q, want %q", stdout, want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("LDAP export wrote credentials to stderr: %q", stderr.String())
			}
		})
	}
}
