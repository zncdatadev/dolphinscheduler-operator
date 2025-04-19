package common

import (
	"context"
	"fmt"
	"maps"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/config"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	s3v1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/s3/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/productlogging"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var configmapLogger = ctrl.Log.WithName("configmap")

func NewConfigMapReconciler(
	ctx context.Context,
	client *client.Client,
	roleGroupInf *reconciler.RoleGroupInfo,
	contaienr util.ContainerComponent,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) reconciler.ResourceReconciler[*builder.ConfigMapBuilder] {
	builder := builder.NewConfigMapBuilder(
		client,
		RoleGroupConfigMapName(roleGroupInf),
		func(o *builder.Options) {
			o.Annotations = roleGroupInf.GetAnnotations()
			o.Labels = roleGroupInf.GetLabels()
		})

	var s3Spec *s3v1alpha1.S3BucketSpec
	owner := client.GetOwnerReference()
	cr := owner.(*dolphinv1alpha1.DolphinschedulerCluster)
	if clusterSpec := cr.Spec.ClusterConfig; clusterSpec != nil {
		s3Spec = clusterSpec.S3
	}

	data := config.GenerateAllFile(ctx, []config.FileContentGenerator{
		&CommonPropertiesGenerator{
			client:    client,
			namespace: client.GetOwnerNamespace(),
			overrides: overrides,
			s3:        s3Spec,
		},
		&LogbackXmlGenerator{
			loggingSpec: roleGroupConfig.Logging,
			container:   contaienr,
		},
	})

	if IsVectorEnable(roleGroupConfig.Logging) {
		ExtendConfigMapDataByVector(ctx, VectorConfigParams{
			Client:        client.GetCtrlClient(),
			ClusterConfig: cr.Spec.ClusterConfig,
			Namespace:     client.GetOwnerNamespace(),
			InstanceName:  client.GetOwnerName(),
			Role:          roleGroupInf.GetRoleName(),
			GroupName:     roleGroupInf.GetGroupName(),
		}, data)
	}
	builder.AddData(data)
	return reconciler.NewGenericResourceReconciler(client, builder)
}

func NewEnvConfigMapReconciler(
	ctx context.Context,
	client *client.Client,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
) reconciler.ResourceReconciler[*builder.ConfigMapBuilder] {
	builder := builder.NewConfigMapBuilder(
		client,
		RoleGroupEnvsConfigMapName(client.GetOwnerName()),
		func(o *builder.Options) {
			o.Annotations = roleGroupInfo.GetAnnotations()
			o.Labels = roleGroupInfo.GetLabels()
		},
	)

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
			overrides:    overrides,
		},
	})
	builder.AddData(data)
	return reconciler.NewGenericResourceReconciler(client, builder)
}

// ----------- common.properties generator -----------
var _ config.FileContentGenerator = &CommonPropertiesGenerator{}

type CommonPropertiesGenerator struct {
	s3     *s3v1alpha1.S3BucketSpec
	client *client.Client

	namespace string
	overrides *commonsv1alpha1.OverridesSpec
}

// FileName implements config.FileContentGenerator.
func (c *CommonPropertiesGenerator) FileName() string {
	return dolphinv1alpha1.DolphinCommonPropertiesName
}

// Generate implements config.FileContentGenerator.
func (c *CommonPropertiesGenerator) Generate(ctx context.Context) (string, error) {
	var commonProperties = make(map[string]string)

	if c.overrides != nil && c.overrides.ConfigOverrides != nil {
		if commonPropertiesOverride, ok := c.overrides.ConfigOverrides[c.FileName()]; ok {
			maps.Copy(commonProperties, commonPropertiesOverride)
		}
	}

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
	loggingSpec *commonsv1alpha1.LoggingSpec
	container   util.ContainerComponent
}

// FileName implements config.FileContentGenerator.
func (l *LogbackXmlGenerator) FileName() string {
	return dolphinv1alpha1.LogbackPropertiesFileName
}

// Generate implements config.FileContentGenerator.
func (l *LogbackXmlGenerator) Generate(ctx context.Context) (string, error) {
	var roleLoggingConfig *commonsv1alpha1.LoggingConfigSpec
	if l.loggingSpec != nil && l.loggingSpec.Containers != nil {
		if containerLoggingSpec, ok := l.loggingSpec.Containers[string(l.container)]; ok {
			roleLoggingConfig = &containerLoggingSpec
		}
	}
	logGenerator, err := NewConfigGenerator(
		roleLoggingConfig,
		string(l.container),
		fmt.Sprintf("%s.log4j.xml", l.container),
		productlogging.LogTypeLogback,
		func(cgo *productlogging.ConfigGeneratorOption) {
			cgo.ConsoleHandlerFormatter = ptr.To(dolphinv1alpha1.ConsoleConversionPattern)
		},
	)
	if err != nil {
		return "", err
	}
	return logGenerator.Content()
}

// ----------- env configmap data generator -----------
var _ config.EnvGenerator = &EnvConfigmapDataGenerator{}

type EnvConfigmapDataGenerator struct {
	client       *client.Client
	namespace    string
	databaseSpec *dolphinv1alpha1.DatabaseSpec
	overrides    *commonsv1alpha1.OverridesSpec
}

// Generate implements config.EnvGenerator.
func (e *EnvConfigmapDataGenerator) Generate(ctx context.Context) (envs map[string]string, err error) {
	envs = make(map[string]string)
	maps.Copy(envs, e.overrides.EnvOverrides)
	if e.databaseSpec == nil {
		return envs, err
	}

	dbInfoExtractor := util.NewDataBaseExtractor(e.client, &e.databaseSpec.ConnectionString).CredentialsInSecret(e.databaseSpec.CredentialsSecret, e.namespace)
	dbConfig, err := dbInfoExtractor.ExtractDatabaseInfo(ctx)
	if err != nil {
		err = errors.WrapWithDetails(err, "failed to get database info", "namespace", e.namespace, "databaseSpec", e.databaseSpec)
		return envs, err
	}
	maps.Copy(envs, map[string]string{
		"DATABASE":                            dbConfig.DbType,
		"SPRING_DATASOURCE_URL":               fmt.Sprintf("jdbc:%s://%s:%s/%s", dbConfig.DbType, dbConfig.Host, dbConfig.Port, dbConfig.DbName),
		"SPRING_DATASOURCE_USERNAME":          dbConfig.Username,
		"SPRING_DATASOURCE_PASSWORD":          dbConfig.Password,
		"SPRING_DATASOURCE_DRIVER-CLASS-NAME": dbConfig.Driver,
	})
	return envs, err
}
