package common

import (
	"context"
	"fmt"
	"maps"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/config"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	loggingv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	ctrl "sigs.k8s.io/controller-runtime"
)

var configmapLogger = ctrl.Log.WithName("configmap")

func NewConfigMapReconciler(
	ctx context.Context,
	client *client.Client,
	roleGroupInf *reconciler.RoleGroupInfo,
	contaienr util.ContainerComponent,
	mergedConfig *dolphinv1alpha1.RoleGroupSpec,
) reconciler.ResourceReconciler[*builder.ConfigMapBuilder] {
	builder := builder.NewConfigMapBuilder(client, RoleGroupConfigMapName(roleGroupInf), roleGroupInf.GetLabels(), roleGroupInf.GetAnnotations())
	var loggingSpec *loggingv1alpha1.LoggingConfigSpec
	if mergedConfig.Config != nil && mergedConfig.Config.Logging != nil && mergedConfig.Config.Logging.Logging != nil {
		loggingSpec = mergedConfig.Config.Logging.Logging
	}

	var s3Spec *s3v1alpha1.S3BucketSpec
	owner := client.GetOwnerReference()
	cr := owner.(*dolphinv1alpha1.DolphinschedulerCluster)
	if clusterSpec := cr.Spec.ClusterConfig; clusterSpec != nil {
		s3Spec = clusterSpec.S3
	}

	data := config.GenerateAllFile(ctx, []config.FileContentGenerator{
		&CommonPropertiesGenerator{
			client:       client,
			namespace:    client.GetOwnerNamespace(),
			mergedConfig: mergedConfig,
			s3:           s3Spec,
		},
		&LogbackXmlGenerator{
			loggingSpec: loggingSpec,
			container:   contaienr,
		},
	})
	builder.AddData(data)
	return reconciler.NewGenericResourceReconciler(client, RoleGroupConfigMapName(roleGroupInf), builder)
}

func NewEnvConfigMapReconciler(
	ctx context.Context,
	client *client.Client,
	mergedConfig *dolphinv1alpha1.RoleGroupSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
) reconciler.ResourceReconciler[*builder.ConfigMapBuilder] {
	builder := builder.NewConfigMapBuilder(client, RoleGroupEnvsConfigMapName(client.GetOwnerName()), roleGroupInfo.GetLabels(), roleGroupInfo.GetAnnotations())

	owner := client.GetOwnerReference()
	cr := owner.(*dolphinv1alpha1.DolphinschedulerCluster)
	var dbSpec *dolphinv1alpha1.DatabaseSpec
	if clusterSpec := cr.Spec.ClusterConfig; clusterSpec != nil {
		dbSpec = clusterSpec.Database
	}

	data := config.GenerateAllEnv(ctx, []config.EnvGenerator{
		&EnvConfigmapDataGenerator{
			client:       client,
			namespace:    client.GetOwnerNamespace(),
			databaseSpec: dbSpec,
			mergedConfig: mergedConfig,
		},
	})
	builder.AddData(data)
	return reconciler.NewGenericResourceReconciler(client, RoleGroupEnvsConfigMapName(client.GetOwnerName()), builder)
}

// ----------- common.properties generator -----------
var _ config.FileContentGenerator = &CommonPropertiesGenerator{}

type CommonPropertiesGenerator struct {
	s3     *s3v1alpha1.S3BucketSpec
	client *client.Client

	namespace    string
	mergedConfig *dolphinv1alpha1.RoleGroupSpec
}

// FileName implements config.FileContentGenerator.
func (c *CommonPropertiesGenerator) FileName() string {
	return dolphinv1alpha1.DolphinCommonPropertiesName
}

// Generate implements config.FileContentGenerator.
func (c *CommonPropertiesGenerator) Generate(ctx context.Context) (string, error) {
	var commonProperties = make(map[string]string)
	maps.Copy(commonProperties, c.mergedConfig.ConfigOverrides.CommonProperties)
	if c.s3 != nil {
		extractor := util.NewS3ConfigExtractor(c.client, c.s3, c.namespace)
		s3Config, err := extractor.GetS3Config(ctx)
		if err != nil {
			configmapLogger.Error(err, "failed to get s3 config", "namespace", c.namespace, "s3BucketSpec", c.s3)
			return "", errors.WrapWithDetails(err, "failed to get s3 config", "namespace", c.namespace, "s3BucketSpec", c.s3)
		}
		var s3Args = make(map[string]string)
		s3Args["resource.storage.type"] = "S3"
		s3Args["resource.aws.access.key.id"] = s3Config.AccessKeyID
		s3Args["resource.aws.region"] = s3Config.Region
		s3Args["resource.aws.s3.bucket.name"] = s3Config.BucketName
		s3Args["resource.aws.s3.endpoint"] = s3Config.Endpoint
		s3Args["resource.aws.secret.access.key"] = s3Config.SecretAccessKey
		maps.Copy(commonProperties, s3Args)
	}
	return util.ToProperties(commonProperties), nil
}

// ----------- logback.xml generator -----------
var _ config.FileContentGenerator = &LogbackXmlGenerator{}

type LogbackXmlGenerator struct {
	loggingSpec *loggingv1alpha1.LoggingConfigSpec
	container   util.ContainerComponent
}

// FileName implements config.FileContentGenerator.
func (l *LogbackXmlGenerator) FileName() string {
	return dolphinv1alpha1.LogbackPropertiesFileName
}

// Generate implements config.FileContentGenerator.
func (l *LogbackXmlGenerator) Generate(ctx context.Context) (string, error) {
	logfileName := fmt.Sprintf("%s.log4j.xml", l.container)
	logbakcConfigGenerator := productlogging.NewLogbackConfigGenerator(l.loggingSpec, string(l.container),
		dolphinv1alpha1.ConsoleConversionPattern, nil, logfileName, l.FileName())
	return logbakcConfigGenerator.Generate(), nil
}

// ----------- env configmap data generator -----------
var _ config.EnvGenerator = &EnvConfigmapDataGenerator{}

type EnvConfigmapDataGenerator struct {
	client       *client.Client
	namespace    string
	databaseSpec *dolphinv1alpha1.DatabaseSpec
	mergedConfig *dolphinv1alpha1.RoleGroupSpec
}

// Generate implements config.EnvGenerator.
func (e *EnvConfigmapDataGenerator) Generate(ctx context.Context) (envs map[string]string, err error) {
	envs = make(map[string]string)
	maps.Copy(envs, e.mergedConfig.EnvOverrides)
	if e.databaseSpec == nil {
		return
	}

	dbInfoExtractor := util.NewDataBaseExtractor(e.client, &e.databaseSpec.ConnectionString).CredentialsInSecret(e.databaseSpec.CredentialsSecret, e.namespace)
	dbConfig, err := dbInfoExtractor.ExtractDatabaseInfo(ctx)
	if err != nil {
		err = errors.WrapWithDetails(err, "failed to get database info", "namespace", e.namespace, "databaseSpec", e.databaseSpec)
		return
	}
	maps.Copy(envs, map[string]string{
		"DATABASE":                            string(dbConfig.DbType),
		"SPRING_DATASOURCE_URL":               fmt.Sprintf("jdbc:%s://%s:%s/%s", dbConfig.DbType, dbConfig.Host, dbConfig.Port, dbConfig.DbName),
		"SPRING_DATASOURCE_USERNAME":          dbConfig.Username,
		"SPRING_DATASOURCE_PASSWORD":          dbConfig.Password,
		"SPRING_DATASOURCE_DRIVER-CLASS-NAME": dbConfig.Driver,
	})
	return
}
