package util

import (
	"context"
	"strings"

	"emperror.dev/errors"
	"github.com/zncdatadev/operator-go/pkg/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConnectionStringAuthUser = "username"
	ConnectionStringAuthPass = "password"
)

type DatabaseConfig struct {
	DbType   string
	Driver   string
	Username string
	Password string
	Host     string
	Port     string
	DbName   string
}

func NewDataBaseExtractor(client *client.Client, connectionString *string) *DataBaseExtractor {
	return &DataBaseExtractor{
		ConnectionString: connectionString,
		Ctx:              context.Background(),
		Client:           client,
	}
}

type DataBaseExtractor struct {
	ConnectionString    *string
	SecretReferenceName *string
	Namespace           *string
	Ctx                 context.Context
	Client              *client.Client
}

func (d *DataBaseExtractor) CredentialsInSecret(secretRefName string, namespace string) *DataBaseExtractor {
	d.SecretReferenceName = &secretRefName
	d.Namespace = &namespace
	return d
}

// extract database info from connection string
func (d *DataBaseExtractor) ExtractDatabaseInfo(ctx context.Context) (*DatabaseConfig, error) {
	if d.ConnectionString == nil {
		return nil, errors.New("connection string is empty")
	}

	// extract database info from connection string
	// e.g. jdbc:postgresql://127.0.0.1:5432/dolphinscheduler?user=root&password=root
	// or jdbc:mysql://127.0.0.1:3306/dolphinscheduler?user=root&password=root
	dbInfo := &DatabaseConfig{}
	parts := strings.Split(*d.ConnectionString, ":")
	if len(parts) < 3 {
		return nil, errors.New("invalid connection string")
	}
	dbInfo.DbType = parts[1]
	driver, err := GetDatabaseDriver(dbInfo.DbType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get database driver")
	} else {
		dbInfo.Driver = driver
	}

	hostParts := strings.Split(parts[2], "/")
	if len(hostParts) < 2 {
		return nil, errors.New("invalid connection string")
	}
	dbInfo.Host = hostParts[2]

	portParts := strings.Split(parts[3], "/")
	dbInfo.Port = portParts[0]

	dbParts := strings.Split(portParts[1], "?")
	dbInfo.DbName = dbParts[0]

	if d.SecretReferenceName != nil {
		// get database credential from secret
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: *d.Namespace,
				Name:      *d.SecretReferenceName,
			},
		}
		var details = map[string]interface{}{
			"namespace":   d.Namespace,
			"secret_name": *d.SecretReferenceName,
		}
		if err := d.Client.GetWithObject(d.Ctx, secret); err != nil {
			return nil, errors.WrapWithDetails(err, "failed to get database secret", details)
		}
		secretData := secret.Data
		if username, ok := secretData[ConnectionStringAuthUser]; ok {
			dbInfo.Username = string(username)
		} else {
			return nil, errors.NewWithDetails("database username is empty", details)
		}
		if password, ok := secretData[ConnectionStringAuthPass]; ok {
			dbInfo.Password = string(password)
		} else {
			return nil, errors.NewWithDetails("database password is empty", details)
		}
	} else {
		// get database credential from connection string
		query := strings.Split(dbParts[1], "&")
		if len(query) == 0 {
			return nil, errors.New("database credential is empty")
		}
		for _, q := range query {
			kv := strings.Split(q, "=")
			if kv[0] == "user" || kv[0] == "username" {
				dbInfo.Username = kv[1]
			}
			if kv[0] == ConnectionStringAuthPass {
				dbInfo.Password = kv[1]
			}
		}
	}
	return dbInfo, nil
}

// get database driver by database type
func GetDatabaseDriver(dbType string) (string, error) {
	switch dbType {
	case "mysql":
		return "com.mysql.cj.jdbc.Driver", nil
	case "postgresql":
		return "org.postgresql.Driver", nil
	default:
		return "", errors.NewWithDetails("unsupported database type to get driver class",
			map[string]interface{}{"type": dbType})
	}
}

func GetDatabaseHost(connectionString string) (host string, port string) {
	// eg. jdbc:postgresql://127.0.0.1:5432/dolphinscheduler?user=root&password=root
	hostPort := strings.Split(connectionString, "//")[1]
	hostPort = strings.Split(hostPort, "/")[0]
	hosts := strings.Split(hostPort, ":")
	if len(hosts) == 2 {
		host = hosts[0]
		port = hosts[1]
	}
	return
}
