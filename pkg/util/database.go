package util

import (
	"context"
	"net/url"
	"strings"

	"emperror.dev/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
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

func NewDataBaseExtractor(client ctrlclient.Client, connectionString *string) *DataBaseExtractor {
	return &DataBaseExtractor{
		ConnectionString: connectionString,
		Client:           client,
	}
}

type DataBaseExtractor struct {
	ConnectionString    *string
	SecretReferenceName *string
	Namespace           *string
	Client              ctrlclient.Client
}

func (d *DataBaseExtractor) CredentialsInSecret(secretRefName string, namespace string) *DataBaseExtractor {
	d.SecretReferenceName = &secretRefName
	d.Namespace = &namespace
	return d
}

func (d *DataBaseExtractor) ExtractDatabaseInfo(ctx context.Context) (*DatabaseConfig, error) {
	if d.ConnectionString == nil {
		return nil, errors.New("connection string is empty")
	}

	dbInfo, err := d.parseConnectionString()
	if err != nil {
		return nil, err
	}

	if err := d.setCredentials(ctx, dbInfo); err != nil {
		return nil, err
	}

	return dbInfo, nil
}

func (d *DataBaseExtractor) parseConnectionString() (*DatabaseConfig, error) {
	databaseURL, err := parseDatabaseURL(*d.ConnectionString)
	if err != nil {
		return nil, err
	}
	if databaseURL.Hostname() == "" {
		return nil, errors.New("database connection string host is empty")
	}
	if strings.Trim(databaseURL.Path, "/") == "" {
		return nil, errors.New("database connection string database name is empty")
	}

	host, port, err := GetDatabaseHost(*d.ConnectionString)
	if err != nil {
		return nil, err
	}

	dbInfo := &DatabaseConfig{
		DbType: databaseURL.Scheme,
		Host:   host,
		Port:   port,
		DbName: strings.TrimPrefix(databaseURL.Path, "/"),
	}

	driver, err := GetDatabaseDriver(dbInfo.DbType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get database driver")
	}
	dbInfo.Driver = driver

	return dbInfo, nil
}

func (d *DataBaseExtractor) setCredentials(ctx context.Context, dbInfo *DatabaseConfig) error {
	if d.SecretReferenceName != nil {
		return d.setCredentialsFromSecret(ctx, dbInfo)
	}
	return d.setCredentialsFromConnectionString(dbInfo)
}

func (d *DataBaseExtractor) setCredentialsFromSecret(ctx context.Context, dbInfo *DatabaseConfig) error {
	secret := &corev1.Secret{}

	details := map[string]interface{}{
		"namespace":   d.Namespace,
		"secret_name": *d.SecretReferenceName,
	}

	key := types.NamespacedName{Namespace: *d.Namespace, Name: *d.SecretReferenceName}
	if err := d.Client.Get(ctx, key, secret); err != nil {
		return errors.WrapWithDetails(err, "failed to get database secret", details)
	}

	secretData := secret.Data
	if username, ok := secretData[ConnectionStringAuthUser]; ok && len(username) > 0 {
		dbInfo.Username = string(username)
	} else {
		return errors.NewWithDetails("database username is empty", details)
	}

	if password, ok := secretData[ConnectionStringAuthPass]; ok && len(password) > 0 {
		dbInfo.Password = string(password)
	} else {
		return errors.NewWithDetails("database password is empty", details)
	}

	return nil
}

func (d *DataBaseExtractor) setCredentialsFromConnectionString(dbInfo *DatabaseConfig) error {
	databaseURL, err := parseDatabaseURL(*d.ConnectionString)
	if err != nil {
		return err
	}

	query := databaseURL.Query()
	dbInfo.Username = query.Get("user")
	if dbInfo.Username == "" {
		dbInfo.Username = query.Get(ConnectionStringAuthUser)
	}
	if dbInfo.Username == "" {
		return errors.New("database username is empty")
	}
	dbInfo.Password = query.Get(ConnectionStringAuthPass)
	if dbInfo.Password == "" {
		return errors.New("database password is empty")
	}

	return nil
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

func GetDatabaseHost(connectionString string) (host string, port string, err error) {
	databaseURL, err := parseDatabaseURL(connectionString)
	if err != nil {
		return "", "", err
	}
	if databaseURL.Hostname() == "" {
		return "", "", errors.New("database connection string host is empty")
	}

	host = databaseURL.Hostname()
	port = databaseURL.Port()
	if port != "" {
		return host, port, nil
	}

	switch databaseURL.Scheme {
	case "postgresql":
		return host, "5432", nil
	case "mysql":
		return host, "3306", nil
	default:
		return "", "", errors.NewWithDetails(
			"database connection string port is empty and no default is known",
			"database_type", databaseURL.Scheme,
		)
	}
}

// CanonicalDatabaseTarget returns a stable, credential-free identity for the database endpoint
// and database name. Fields are NUL-delimited to avoid ambiguous concatenation. The value is
// intended to be hashed before it is persisted in Kubernetes metadata.
func CanonicalDatabaseTarget(connectionString string) (string, error) {
	extractor := &DataBaseExtractor{ConnectionString: &connectionString}
	dbInfo, err := extractor.parseConnectionString()
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		strings.ToLower(dbInfo.DbType),
		strings.ToLower(dbInfo.Host),
		dbInfo.Port,
		dbInfo.DbName,
	}, "\x00"), nil
}

// parseDatabaseURL deliberately discards net/url's error value: url.Error includes the original
// URL and would leak legacy inline JDBC credentials through reconcile errors, status and events.
func parseDatabaseURL(connectionString string) (*url.URL, error) {
	if !strings.HasPrefix(connectionString, "jdbc:") {
		return nil, errors.New("database connection string must start with jdbc:")
	}
	databaseURL, err := url.Parse(strings.TrimPrefix(connectionString, "jdbc:"))
	if err != nil {
		return nil, errors.New("failed to parse database connection string")
	}
	return databaseURL, nil
}
