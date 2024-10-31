package common

import (
	"maps"
	"reflect"
	"strconv"
	"time"

	"emperror.dev/errors"
	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/config"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var configLogger = ctrl.Log.WithName("config-logger")

const (
	DefaultServerGrace = 120
)

var _ config.Configuration = &DolphinSchedulerConfig{}

func DefaultConfig(role util.Role, crName string) *DolphinSchedulerConfig {
	var cpuMin, cpuMax, memoryLimit, storage resource.Quantity
	switch role {
	case Master:
		cpuMin = parseQuantity("300m")
		cpuMax = parseQuantity("500m")
		memoryLimit = parseQuantity("800Mi")
		storage = parseQuantity("1Gi")
	case Worker:
		cpuMin = parseQuantity("400m")
		cpuMax = parseQuantity("600m")
		memoryLimit = parseQuantity("1Gi")
		storage = parseQuantity("2Gi")
	case Api:
		cpuMin = parseQuantity("400m")
		cpuMax = parseQuantity("700m")
		memoryLimit = parseQuantity("1Gi")
		storage = parseQuantity("1Gi")
	case Alerter:
		cpuMin = parseQuantity("300m")
		cpuMax = parseQuantity("400m")
		memoryLimit = parseQuantity("800Mi")
		storage = parseQuantity("1Gi")
	default:
		panic("invalid role")
	}

	resources := &commonsv1alpha1.ResourcesSpec{
		CPU: &commonsv1alpha1.CPUResource{
			Min: cpuMin,
			Max: cpuMax,
		},
		Memory: &commonsv1alpha1.MemoryResource{
			Limit: memoryLimit,
		},
		Storage: &commonsv1alpha1.StorageResource{
			Capacity: storage,
		},
	}

	return &DolphinSchedulerConfig{
		resources: resources,
		common: &GeneralNodeConfig{
			Affinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 70,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: "kubernetes.io/hostname",
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										constants.LabelKubernetesInstance:  crName,
										constants.LabelKubernetesComponent: string(role),
									},
								},
							},
						},
					},
				},
			}, // TODO: refactor with affinity builder of operator-go in the future, here handled it simplely now.
			gracefulShutdownTimeoutSeconds: DefaultServerGrace * time.Second,
		},
	}
}

type DolphinSchedulerConfig struct {
	role      util.Role
	resources *commonsv1alpha1.ResourcesSpec
	// Logging Logging `json:"logging,omitempty"`
	common *GeneralNodeConfig
}

type GeneralNodeConfig struct {
	Affinity *corev1.Affinity

	gracefulShutdownTimeoutSeconds time.Duration
}

func (G *GeneralNodeConfig) GetgracefulShutdownTimeoutSeconds() *string {
	seconds := G.gracefulShutdownTimeoutSeconds.Seconds()
	v := strconv.Itoa(int(seconds)) + "s"
	return &v
}

// ComputeClient implements config.Configuration.
func (c *DolphinSchedulerConfig) ComputeCli() (map[string]string, error) {
	return map[string]string{}, nil
}

// ComputeEnv implements config.Configuration.
func (c *DolphinSchedulerConfig) ComputeEnv() (map[string]string, error) {
	return map[string]string{
		// "DATA_BASEDIR_PATH":     "/tmp/dolphinscheduler",
		// "DATAX_LAUNCHER":        "/opt/soft/datax/bin/datax.py",
		// "DOLPHINSCHEDULER_OPTS": "",
		// "FLINK_HOME":            "/opt/soft/flink",
		// "HADOOP_CONF_DIR":       "/opt/soft/hadoop/etc/hadoop",
		// "HADOOP_HOME":           "/opt/soft/hadoop",
		// "HIVE_HOME":             "/opt/soft/hive",
		// "JAVA_HOME":                "/opt/java/openjdk",
		// "PYTHON_LAUNCHER":          "/usr/bin/python/bin/python3",
		// "RESOURCE_UPLOAD_PATH":     "/dolphinscheduler",
		// "SPARK_HOME":               "/opt/soft/spark",
		"TZ":                       "Asia/Shanghai",
		"SPRING_JACKSON_TIME_ZONE": "Asia/Shanghai",
	}, nil
}

// ComputeFile implements config.Configuration.
func (c *DolphinSchedulerConfig) ComputeFile() (map[string]interface{}, error) {
	commonProperties := map[string]string{
		"alert.rpc.port":                               "50052",
		"appId.collect":                                "log",
		"conda.path":                                   "/opt/anaconda3/etc/profile.d/conda.sh",
		"data.basedir.path":                            "/tmp/dolphinscheduler",
		"datasource.encryption.enable":                 "false",
		"datasource.encryption.salt":                   "!@#$%^&*",
		"development.state":                            "false",
		"hadoop.security.authentication.startup.state": "false",
		"java.security.krb5.conf.path":                 "/opt/krb5.conf",
		"kerberos.expire.time":                         "2",
		"login.user.keytab.path":                       "/opt/hdfs.headless.keytab",
		"login.user.keytab.username":                   "hdfs-mycluster@ESZ.COM",
		"ml.mlflow.preset_repository":                  "https://github.com/apache/dolphinscheduler-mlflow",
		"ml.mlflow.preset_repository_version":          "main",
		"resource.alibaba.cloud.access.key.id":         "<your-access-key-id>",
		"resource.alibaba.cloud.access.key.secret":     "<your-access-key-secret>",
		"resource.alibaba.cloud.oss.bucket.name":       "dolphinscheduler",
		"resource.alibaba.cloud.oss.endpoint":          "https://oss-cn-hangzhou.aliyuncs.com",
		"resource.alibaba.cloud.region":                "cn-hangzhou",
		"resource.azure.client.id":                     "minioadmin",
		"resource.azure.client.secret":                 "minioadmin",
		"resource.azure.subId":                         "minioadmin",
		"resource.azure.tenant.id":                     "minioadmin",
		"resource.hdfs.fs.defaultFS":                   "hdfs://mycluster:8020",
		"resource.hdfs.root.user":                      "hdfs",
		"resource.manager.httpaddress.port":            "8088",
		"resource.storage.type":                        "LOCAL",
		"resource.storage.upload.base.path":            "/dolphinscheduler",
		"sudo.enable":                                  "true",
		"support.hive.oneSession":                      "false",
		"task.resource.limit.state":                    "false",
		"yarn.application.status.address":              "http://ds1:%s/ws/v1/cluster/apps/%s",
		"yarn.job.history.status.address":              "http://ds1:19888/ws/v1/history/mapreduce/jobs/%s",
		"yarn.resourcemanager.ha.rm.ids":               "192.168.xx.xx,192.168.xx.xx",
	}
	ApiServerDefaultaApplicationYaml := `security:
  authentication:
    type: PASSWORD
    oauth2:
      enable: false
      provider:
	`
	configs := map[string]interface{}{
		dolphinv1alpha1.DolphinCommonPropertiesName: commonProperties,
	}
	if c.role == Api {
		maps.Copy(configs, map[string]interface{}{
			dolphinv1alpha1.ApplicationServerConfigFileName: ApiServerDefaultaApplicationYaml,
		})
	}
	return configs, nil
}

// merge defaultConfig
func (c *DolphinSchedulerConfig) Merge(mergedCfg *dolphinv1alpha1.RoleGroupSpec) {

	if mergedCfg.Config == nil {
		mergedCfg.Config = &dolphinv1alpha1.ConfigSpec{}
	}

	// mergedresources
	if mergedresources := mergedCfg.Config.Resources; mergedresources == nil {
		mergedCfg.Config.Resources = c.resources
	} else {
		if mergedCpu := mergedresources.CPU; mergedCpu == nil {
			mergedCfg.Config.Resources.CPU = c.resources.CPU
		}
		if mergedMemory := mergedresources.Memory; mergedMemory == nil {
			mergedCfg.Config.Resources.Memory = c.resources.Memory
		}
		if mergedStorage := mergedresources.Storage; mergedStorage == nil {
			mergedCfg.Config.Resources.Storage = c.resources.Storage
		}
	}

	//affinity
	if mergedCfg.Config.Affinity == nil {
		mergedCfg.Config.Affinity = c.common.Affinity
	}

	// gracefulShutdownTimeoutSeconds
	if mergedCfg.Config.GracefulShutdownTimeout == nil {
		mergedCfg.Config.GracefulShutdownTimeout = c.common.GetgracefulShutdownTimeoutSeconds()
	}

	// common properties
	fileConfig, _ := c.ComputeFile()
	if mergedCfg.ConfigOverrides == nil {
		mergedCfg.ConfigOverrides = &dolphinv1alpha1.ConfigOverridesSpec{}
	}
	if mergedCfg.ConfigOverrides.CommonProperties == nil {
		if commonArgs, ok := fileConfig[dolphinv1alpha1.DolphinCommonPropertiesName]; ok {
			mergedCfg.ConfigOverrides.CommonProperties = toMap(commonArgs)
		}
	} else {
		src := mergedCfg.ConfigOverrides.CommonProperties
		if commonArgs, ok := fileConfig[dolphinv1alpha1.DolphinCommonPropertiesName]; ok {
			dist := toMap(commonArgs)
			maps.Copy(dist, src) // cr define overrdie default
			mergedCfg.ConfigOverrides.CommonProperties = dist
		}
	}

	// envOverride
	envConfig, _ := c.ComputeEnv()
	if mergedCfg.ConfigOverrides == nil {
		mergedCfg.EnvOverrides = envConfig
	} else {
		src := mergedCfg.EnvOverrides
		dist := envConfig
		maps.Copy(dist, src) // cr define overrdie default
		mergedCfg.EnvOverrides = dist
	}

	// do other merge ...

}

func parseQuantity(q string) resource.Quantity {
	r := resource.MustParse(q)
	return r
}

func toMap(i interface{}) map[string]string {
	m := make(map[string]string)
	if mapstring, ok := i.(map[string]string); ok {
		for k, v := range mapstring {
			m[k] = v
		}
	} else {
		configLogger.Error(errors.New("parse config error, config is not a map[string]string type"), "parse config error", "actual type", reflect.TypeOf(i))
	}
	return m
}
