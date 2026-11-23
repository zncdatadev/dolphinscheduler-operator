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

package controller

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	opgos3 "github.com/zncdatadev/operator-go/pkg/s3"
)

func TestS3CredentialsExportScript(t *testing.T) {
	credentialsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(credentialsDir, opgos3.AccessKeyFile), []byte("access-key"), 0o600); err != nil {
		t.Fatalf("write access-key fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credentialsDir, opgos3.SecretKeyFile), []byte("secret-key"), 0o600); err != nil {
		t.Fatalf("write secret-key fixture: %v", err)
	}

	script := s3CredentialsExportScriptForMount(credentialsDir) + `
printf '%s|%s|%s|%s' "$AWS_ACCESS_KEY_ID" "$AWS_SECRET_ACCESS_KEY" "$RESOURCE_AWS_ACCESS_KEY_ID" "$RESOURCE_AWS_SECRET_ACCESS_KEY"`
	// #nosec G204 -- this executes the generated fixed script against fake credentials only.
	cmd := exec.Command("/bin/bash", "-euo", "pipefail", "-c", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("execute S3 export script: %v (stderr: %q)", err, stderr.String())
	}
	if got, want := string(stdout), "access-key|secret-key|access-key|secret-key"; got != want {
		t.Fatalf("S3 export output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("S3 export wrote credentials to stderr: %q", stderr.String())
	}
}

func TestS3CredentialsExportScriptRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name        string
		writeAccess bool
		access      string
		writeSecret bool
		secret      string
	}{
		{name: "missing access key", writeSecret: true, secret: "do-not-leak"},
		{name: "empty access key", writeAccess: true, writeSecret: true, secret: "do-not-leak"},
		{name: "missing secret key", writeAccess: true, access: "access-key"},
		{name: "empty secret key", writeAccess: true, access: "access-key", writeSecret: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credentialsDir := t.TempDir()
			if tt.writeAccess {
				if err := os.WriteFile(filepath.Join(credentialsDir, opgos3.AccessKeyFile), []byte(tt.access), 0o600); err != nil {
					t.Fatalf("write access-key fixture: %v", err)
				}
			}
			if tt.writeSecret {
				if err := os.WriteFile(filepath.Join(credentialsDir, opgos3.SecretKeyFile), []byte(tt.secret), 0o600); err != nil {
					t.Fatalf("write secret-key fixture: %v", err)
				}
			}

			script := s3CredentialsExportScriptForMount(credentialsDir) + "\nprintf reached"
			// #nosec G204 -- this executes the generated fixed script against fake credentials only.
			cmd := exec.Command("/bin/bash", "-euo", "pipefail", "-c", script)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("S3 export script succeeded for %s", tt.name)
			}
			if strings.Contains(string(output), "reached") || strings.Contains(string(output), "do-not-leak") {
				t.Fatalf("S3 export script continued or leaked a credential: %q", output)
			}
		})
	}
}
