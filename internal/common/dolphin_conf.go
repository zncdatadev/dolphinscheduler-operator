package common

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
)

var _ resource.EnvGenerator = &EnvPropertiesGenerator{}

type EnvPropertiesGenerator struct {
}

func (e EnvPropertiesGenerator) Generate() map[string]string {
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
}

func (c *ConfigPropertiesGenerator) Generate() string {
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
resource.aws.access.key.id=minioadmin
resource.aws.region=ca-central-1
resource.aws.s3.bucket.name=dolphinscheduler
resource.aws.s3.endpoint=http://dolphinscheduler-minio:9000
resource.aws.secret.access.key=minioadmin
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
