package resource

import (
	"fmt"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	commonsv1alph1 "github.com/zncdata-labs/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdata-labs/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

type DataBaseType string

const (
	Postgres DataBaseType = "postgresql"
	Mysql    DataBaseType = "mysql"
	Derby    DataBaseType = "derby"
	Unknown  DataBaseType = "unknown"
)

const (
	DbUsernameName = "USERNAME"
	DbPasswordName = "PASSWORD"
)

type DatabaseCredential struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type DatabaseParams struct {
	DbType   DataBaseType
	Driver   string
	Username string
	Password string
	Host     string
	Port     string
	DbName   string
}

func NewDatabaseParams(
	Driver string,
	Username string,
	Password string,
	Host string,
	Port string,
	DbName string) *DatabaseParams {
	var dbType DataBaseType
	if strings.Contains(Driver, "postgresql") {
		dbType = Postgres
	}
	if strings.Contains(Driver, "mysql") {
		dbType = Mysql
	}
	if strings.Contains(Driver, "derby") {
		dbType = Derby
	}
	if Driver == "" {
		dbType = Unknown
	}
	return &DatabaseParams{
		DbType:   dbType,
		Driver:   Driver,
		Username: Username,
		Password: Password,
		Host:     Host,
		Port:     Port,
		DbName:   DbName,
	}
}

type DatabaseConfiguration struct {
	DbReference    *string
	DbInline       *DatabaseParams
	ResourceClient core.ResourceClient
}

func (d *DatabaseConfiguration) GetRefDatabaseName() string {
	return *d.DbReference
}

func (d *DatabaseConfiguration) GetRefDatabase() (commonsv1alph1.Database, error) {
	databaseCR := &commonsv1alph1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      d.GetRefDatabaseName(),
		},
	}
	if err := d.ResourceClient.Get(databaseCR); err != nil {
		return commonsv1alph1.Database{}, err
	}
	return *databaseCR, nil
}

func (d *DatabaseConfiguration) GetRefDatabaseConnection(name string) (commonsv1alph1.DatabaseConnection, error) {
	databaseConnectionCR := &commonsv1alph1.DatabaseConnection{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      name,
		},
	}

	if err := d.ResourceClient.Get(databaseConnectionCR); err != nil {
		return commonsv1alph1.DatabaseConnection{}, err
	}
	return *databaseConnectionCR, nil
}

func (d *DatabaseConfiguration) GetCredential(name string) (*DatabaseCredential, error) {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: d.ResourceClient.Namespace,
			Name:      name,
		},
	}

	if err := d.ResourceClient.Get(secret); err != nil {
		return nil, err
	}

	username, err := util.Base64[[]byte]{Data: secret.Data[DbUsernameName]}.Decode()
	if err != nil {
		return nil, err
	}

	password, err := util.Base64[[]byte]{Data: secret.Data[DbPasswordName]}.Decode()
	if err != nil {
		return nil, err
	}

	return &DatabaseCredential{
		Username: string(username),
		Password: string(password),
	}, nil
}

func (d *DatabaseConfiguration) getDatabaseParamsFromResource() (*DatabaseParams, error) {
	db, err := d.GetRefDatabase()
	if err != nil {
		return nil, err
	}
	credential := &DatabaseCredential{}

	if db.Spec.Credential.ExistSecret != "" {
		c, err := d.GetCredential(db.Spec.Credential.ExistSecret)
		if err != nil {
			return nil, err
		}
		credential = c
	} else {
		credential.Username = db.Spec.Credential.Username
		credential.Password = db.Spec.Credential.Password
	}

	dbConnection, err := d.GetRefDatabaseConnection(db.Spec.Reference)
	if err != nil {
		return nil, err
	}

	dbParams := &DatabaseParams{
		Username: credential.Username,
		Password: credential.Password,
	}

	provider := dbConnection.Spec.Provider

	if provider.Postgres != nil {
		dbParams.DbType = Postgres
		dbParams.Driver = provider.Postgres.Driver
		dbParams.Host = provider.Postgres.Host
		dbParams.Port = strconv.Itoa(provider.Postgres.Port)
		dbParams.DbName = db.Spec.DatabaseName
		return dbParams, nil
	} else if provider.Mysql != nil {
		dbParams.DbType = Mysql
		dbParams.Driver = provider.Mysql.Driver
		dbParams.Host = provider.Mysql.Host
		dbParams.Port = strconv.Itoa(provider.Mysql.Port)
		dbParams.DbName = db.Spec.DatabaseName
		return dbParams, nil
	} else {
		return &DatabaseParams{
			DbType:   Derby,
			Driver:   "",
			Username: "",
			Password: "",
			Host:     "",
			Port:     "",
			DbName:   "",
		}, nil
	}
}

func (d *DatabaseConfiguration) getDatabaseParamsFromInline() (*DatabaseParams, error) {
	return d.DbInline, nil
}

func (d *DatabaseConfiguration) GetDatabaseParams() (*DatabaseParams, error) {
	if d.DbReference != nil {
		return d.getDatabaseParamsFromResource()
	}
	if d.DbInline != nil {
		return d.getDatabaseParamsFromInline()
	}
	return nil, fmt.Errorf("invalid database configuration, dbReference and dbInline cannot be empty at the same time")
}

func (d *DatabaseConfiguration) GetURI() (string, error) {
	if d.DbReference != nil {
		if refData, err := d.getDatabaseParamsFromResource(); err != nil {
			return "", err
		} else {
			return toUri(*refData), nil
		}
	}
	if d.DbInline != nil {
		return toUri(*d.DbInline), nil
	}
	return "", fmt.Errorf("invalid database configuration, dbReference and dbInline cannot be empty at the same time")
}

func toUri(params DatabaseParams) string {
	var jdbcPrefix string
	switch params.DbType {
	case Mysql:
		jdbcPrefix = "jdbc:mysql"
	case Postgres:
		jdbcPrefix = "jdbc:postgresql"
	case Derby:
		jdbcPrefix = "jdbc:derby"
	default:
		jdbcPrefix = fmt.Sprintf("unknown jdbc prefix for driver %s", params.DbType)
	}
	return fmt.Sprintf("%s://%s:%s/%s",
		jdbcPrefix,
		params.Host,
		params.Port,
		params.DbName,
	)
}
