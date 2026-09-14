package util_test

// Extracts database info from a valid connection string
import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetDatabaseHost(t *testing.T) {
	tests := []struct {
		name       string
		connection string
		wantHost   string
		wantPort   string
		wantErr    bool
	}{
		{
			name:       "PostgreSQL custom port",
			connection: "jdbc:postgresql://postgresql.example.com:15432/dolphinscheduler",
			wantHost:   "postgresql.example.com",
			wantPort:   "15432",
		},
		{
			name:       "PostgreSQL default port",
			connection: "jdbc:postgresql://postgresql.example.com/dolphinscheduler",
			wantHost:   "postgresql.example.com",
			wantPort:   "5432",
		},
		{
			name:       "MySQL default port",
			connection: "jdbc:mysql://mysql.example.com/dolphinscheduler",
			wantHost:   "mysql.example.com",
			wantPort:   "3306",
		},
		{
			name:       "IPv6 host",
			connection: "jdbc:postgresql://[2001:db8::1]:5433/dolphinscheduler",
			wantHost:   "2001:db8::1",
			wantPort:   "5433",
		},
		{
			name:       "missing JDBC prefix",
			connection: "postgresql://postgresql:5432/dolphinscheduler",
			wantErr:    true,
		},
		{
			name:       "missing host",
			connection: "jdbc:postgresql:///dolphinscheduler",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := util.GetDatabaseHost(tt.connection)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetDatabaseHost(%q) error = nil, want an error", tt.connection)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetDatabaseHost(%q) error = %v", tt.connection, err)
			}
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("GetDatabaseHost(%q) = (%q, %q), want (%q, %q)",
					tt.connection, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestDatabaseParseErrorsRedactInlineCredentials(t *testing.T) {
	connection := "jdbc:postgresql://postgresql/%ZZ?user=db-user&password=do-not-leak"
	_, _, hostErr := util.GetDatabaseHost(connection)
	if hostErr == nil {
		t.Fatal("GetDatabaseHost() error = nil, want an error")
	}
	if strings.Contains(hostErr.Error(), "do-not-leak") || strings.Contains(hostErr.Error(), connection) {
		t.Fatalf("GetDatabaseHost() leaked the connection string: %v", hostErr)
	}

	_, extractErr := util.NewDataBaseExtractor(nil, &connection).ExtractDatabaseInfo(context.Background())
	if extractErr == nil {
		t.Fatal("ExtractDatabaseInfo() error = nil, want an error")
	}
	if strings.Contains(extractErr.Error(), "do-not-leak") || strings.Contains(extractErr.Error(), connection) {
		t.Fatalf("ExtractDatabaseInfo() leaked the connection string: %v", extractErr)
	}
}

func TestCanonicalDatabaseTargetExcludesCredentials(t *testing.T) {
	first := "jdbc:postgresql://POSTGRESQL/dolphinscheduler?user=first&password=one"
	second := "jdbc:postgresql://postgresql:5432/dolphinscheduler?user=second&password=two"
	firstTarget, err := util.CanonicalDatabaseTarget(first)
	if err != nil {
		t.Fatalf("CanonicalDatabaseTarget(first) error = %v", err)
	}
	secondTarget, err := util.CanonicalDatabaseTarget(second)
	if err != nil {
		t.Fatalf("CanonicalDatabaseTarget(second) error = %v", err)
	}
	if firstTarget != secondTarget {
		t.Fatalf("targets differ only by credentials/default port: %q != %q", firstTarget, secondTarget)
	}

	otherTarget, err := util.CanonicalDatabaseTarget(
		"jdbc:postgresql://postgresql:5432/another-database?user=first&password=one")
	if err != nil {
		t.Fatalf("CanonicalDatabaseTarget(other) error = %v", err)
	}
	if firstTarget == otherTarget {
		t.Fatal("database name change did not change canonical target")
	}
}

func TestDataBaseExtractorInlineCredentials(t *testing.T) {
	tests := []struct {
		name       string
		connection string
		wantUser   string
		wantPass   string
		wantHost   string
		wantPort   string
		wantErr    bool
	}{
		{
			name:       "URL-decoded credentials with an equals sign",
			connection: "jdbc:postgresql://[2001:db8::1]/dolphinscheduler?username=db-user&password=p%40ss%3Dword",
			wantUser:   "db-user",
			wantPass:   "p@ss=word",
			wantHost:   "2001:db8::1",
			wantPort:   "5432",
		},
		{
			name:       "missing username",
			connection: "jdbc:postgresql://postgresql/dolphinscheduler?password=secret",
			wantErr:    true,
		},
		{
			name:       "missing password",
			connection: "jdbc:postgresql://postgresql/dolphinscheduler?user=db-user",
			wantErr:    true,
		},
		{
			name:       "missing database name",
			connection: "jdbc:postgresql://postgresql:5432?user=db-user&password=secret",
			wantErr:    true,
		},
		{
			name:       "unsupported database type",
			connection: "jdbc:oracle://oracle:1521/dolphinscheduler?user=db-user&password=secret",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbInfo, err := util.NewDataBaseExtractor(nil, &tt.connection).
				ExtractDatabaseInfo(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ExtractDatabaseInfo(%q) error = nil, want an error", tt.connection)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractDatabaseInfo(%q) error = %v", tt.connection, err)
			}
			if dbInfo.Username != tt.wantUser || dbInfo.Password != tt.wantPass ||
				dbInfo.Host != tt.wantHost || dbInfo.Port != tt.wantPort {
				t.Fatalf("ExtractDatabaseInfo(%q) = %#v", tt.connection, dbInfo)
			}
		})
	}
}

func TestDataBaseExtractorRejectsEmptySecretValues(t *testing.T) {
	connection := "jdbc:postgresql://postgresql:5432/dolphinscheduler"
	for _, emptyKey := range []string{util.ConnectionStringAuthUser, util.ConnectionStringAuthPass} {
		t.Run(emptyKey, func(t *testing.T) {
			data := map[string][]byte{
				util.ConnectionStringAuthUser: []byte("db-user"),
				util.ConnectionStringAuthPass: []byte("db-password"),
			}
			data[emptyKey] = []byte{}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "default"},
				Data:       data,
			}
			client := fakeclient.NewClientBuilder().WithScheme(clientgoscheme.Scheme).WithObjects(secret).Build()

			_, err := util.NewDataBaseExtractor(client, &connection).
				CredentialsInSecret(secret.Name, secret.Namespace).
				ExtractDatabaseInfo(context.Background())
			if err == nil {
				t.Fatalf("ExtractDatabaseInfo() error = nil when %s is empty", emptyKey)
			}
		})
	}
}

var _ = Describe("DataBaseExtractor", func() {
	Context("when extracting database info", func() {
		It("should extract database info from a valid connection string", func() {
			// given
			connectionString := "jdbc:postgresql://127.0.0.1:5432/dolphinscheduler?user=root&password=root"
			d := util.NewDataBaseExtractor(k8sClient, &connectionString)

			// when
			dbInfo, err := d.ExtractDatabaseInfo(context.Background())

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(dbInfo).NotTo(BeNil())
			Expect(dbInfo.DbType).To(Equal("postgresql"))
			Expect(dbInfo.Driver).To(Equal("org.postgresql.Driver"))
			Expect(dbInfo.Host).To(Equal("127.0.0.1"))
			Expect(dbInfo.Port).To(Equal("5432"))
			Expect(dbInfo.DbName).To(Equal("dolphinscheduler"))
			Expect(dbInfo.Username).To(Equal("root"))
			Expect(dbInfo.Password).To(Equal("root"))
		})

		It("should return error when connection string is nil", func() {
			// given
			d := &util.DataBaseExtractor{
				ConnectionString: nil,
			}

			// when
			dbInfo, err := d.ExtractDatabaseInfo(context.Background())

			// then
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("connection string is empty"))
			Expect(dbInfo).To(BeNil())
		})
	})
})
