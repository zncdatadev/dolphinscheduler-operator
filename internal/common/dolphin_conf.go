package common

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ resource.EnvGenerator = &EnvPropertiesGenerator{}

type EnvPropertiesGenerator struct {
}

func (e *EnvPropertiesGenerator) Generate() map[string]string {
	return map[string]string{
		"DATA_BASEDIR_PATH":     "/tmp/dolphinscheduler",
		"DATAX_LAUNCHER":        "/opt/soft/datax/bin/datax.py",
		"DOLPHINSCHEDULER_OPTS": "",
		"FLINK_HOME":            "/opt/soft/flink",
		"HADOOP_CONF_DIR":       "/opt/soft/hadoop/etc/hadoop",
		"HADOOP_HOME":           "/opt/soft/hadoop",
		"HIVE_HOME":             "/opt/soft/hive",
		"JAVA_HOME":             "/opt/java/openjdk",
		"PYTHON_LAUNCHER":       "/usr/bin/python/bin/python3",
		"RESOURCE_UPLOAD_PATH":  "/dolphinscheduler",
		"SPARK_HOME":            "/opt/soft/spark",
	}
}

var _ resource.FileContentGenerator = &ConfigPropertiesGenerator{}

type ConfigPropertiesGenerator struct {
	s3Spec    *dolphinv1alpha1.S3BucketSpec
	client    client.Client
	namespace string
}

func NewConfigPropertiesGenerator(s3Spec *dolphinv1alpha1.S3BucketSpec,
	client client.Client, namespace string) *ConfigPropertiesGenerator {
	return &ConfigPropertiesGenerator{
		s3Spec:    s3Spec,
		client:    client,
		namespace: namespace,
	}
}

func (c *ConfigPropertiesGenerator) ExtractS3eReference(s3Spec *dolphinv1alpha1.S3BucketSpec,
	ctx context.Context, client client.Client, namespace string) *resource.S3Params {
	s3 := resource.S3Configuration{
		S3Reference:    s3Spec.Reference,
		ResourceClient: core.NewResourceClient(ctx, client, namespace),
	}
	if inlineS3 := s3Spec.Inline; inlineS3 != nil {
		s3.S3Inline = &resource.S3Params{
			AccessKey: inlineS3.AccessKey,
			SecretKey: inlineS3.SecretKey,
			Endpoint:  inlineS3.Endpoints,
			Bucket:    inlineS3.Bucket,
			Region:    inlineS3.Region,
			SSL:       inlineS3.SSL,
			PathStyle: inlineS3.PathStyle,
		}
	}
	params, err := s3.GetS3Params()
	if err != nil {
		panic(err)
	}
	return params
}
func (c *ConfigPropertiesGenerator) Generate() string {
	if c.s3Spec == nil {
		panic("s3Spec is nil")
	}
	s3Params := c.ExtractS3eReference(c.s3Spec, context.Background(), c.client, c.namespace)
	//resource.aws.access.key.id=minioadmin
	//resource.aws.region=ca-central-1
	//resource.aws.s3.bucket.name=dolphinscheduler
	//resource.aws.s3.endpoint=http://dolphinscheduler-minio:9000
	//resource.aws.secret.access.key=minioadmin
	var s3Resource string
	if s3Params != nil {
		s3Resource = `resource.aws.access.key.id=` + s3Params.AccessKey + `
resource.aws.region=` + s3Params.Region + `
resource.aws.s3.bucket.name=` + s3Params.Bucket + `
resource.aws.s3.endpoint=` + s3Params.Endpoint + `
resource.aws.secret.access.key=` + s3Params.SecretKey
	}
	return `alert.rpc.port=50052
appId.collect=log
conda.path=/opt/anaconda3/etc/profile.d/conda.sh
data.basedir.path=/tmp/dolphinscheduler
datasource.encryption.enable=false
datasource.encryption.salt=!@#$%^&*
development.state=false
hadoop.security.authentication.startup.state=false
java.security.krb5.conf.path=/opt/krb5.conf
kerberos.expire.time=2
login.user.keytab.path=/opt/hdfs.headless.keytab
login.user.keytab.username=hdfs-mycluster@ESZ.COM
ml.mlflow.preset_repository=https://github.com/apache/dolphinscheduler-mlflow
ml.mlflow.preset_repository_version=main
resource.alibaba.cloud.access.key.id=<your-access-key-id>
resource.alibaba.cloud.access.key.secret=<your-access-key-secret>
resource.alibaba.cloud.oss.bucket.name=dolphinscheduler
resource.alibaba.cloud.oss.endpoint=https://oss-cn-hangzhou.aliyuncs.com
resource.alibaba.cloud.region=cn-hangzhou
` + s3Resource + `
resource.azure.client.id=minioadmin
resource.azure.client.secret=minioadmin
resource.azure.subId=minioadmin
resource.azure.tenant.id=minioadmin
resource.hdfs.fs.defaultFS=hdfs://mycluster:8020
resource.hdfs.root.user=hdfs
resource.manager.httpaddress.port=8088
resource.storage.type=S3
resource.storage.upload.base.path=/dolphinscheduler
sudo.enable=true
support.hive.oneSession=false
task.resource.limit.state=false
yarn.application.status.address=http://ds1:%s/ws/v1/cluster/apps/%s
yarn.job.history.status.address=http://ds1:19888/ws/v1/history/mapreduce/jobs/%s
yarn.resourcemanager.ha.rm.ids=192.168.xx.xx,192.168.xx.xx
`
}

func (c *ConfigPropertiesGenerator) FileName() string {
	return dolphinv1alpha1.DolphinCommonPropertiesName
}
