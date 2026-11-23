package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultEnabled         = "true"
	defaultDisabled        = "false"
	defaultMinioCredential = "minioadmin"

	// defaultLoadProtectionThreshold is the historical default for the master/worker
	// load-protection percentage thresholds.
	defaultLoadProtectionThreshold = "0.7"

	// defaultGracefulShutdown is the product default terminationGracePeriodSeconds (as a
	// duration string), applied when neither the role nor the role group sets one.
	defaultGracefulShutdown = "120s"

	// envJavaOpts is the JVM options env var shared by the master/api/alert defaults.
	envJavaOpts = "JAVA_OPTS"

	// defaultRoleGroupName is the conventional role group name.
	defaultRoleGroupName = "default"

	// quantity400m / quantity1Gi are resource quantities shared by several role defaults.
	quantity400m = "400m"
	quantity1Gi  = "1Gi"
)

// defaultCommonProperties returns the DolphinScheduler common.properties defaults, verbatim
// from the pre-migration operator. User configOverrides["common.properties"] now really win
// over these (the legacy code inverted that merge); the S3 overlay is applied on top in the
// handler, matching the legacy precedence where S3-derived keys were written last.
func defaultCommonProperties() map[string]string {
	return map[string]string{
		"alert.rpc.port":                               "50052",
		"appId.collect":                                "log",
		"conda.path":                                   "/opt/anaconda3/etc/profile.d/conda.sh",
		"data.basedir.path":                            workerDataMountPath,
		"datasource.encryption.enable":                 defaultDisabled,
		"datasource.encryption.salt":                   "!@#$%^&*",
		"development.state":                            defaultDisabled,
		"hadoop.security.authentication.startup.state": defaultDisabled,
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
		"resource.azure.client.id":                     defaultMinioCredential,
		"resource.azure.client.secret":                 defaultMinioCredential,
		"resource.azure.subId":                         defaultMinioCredential,
		"resource.azure.tenant.id":                     defaultMinioCredential,
		"resource.hdfs.fs.defaultFS":                   "hdfs://mycluster:8020",
		"resource.hdfs.root.user":                      "hdfs",
		"resource.manager.httpaddress.port":            "8088",
		"resource.storage.type":                        "LOCAL",
		"resource.storage.upload.base.path":            "/dolphinscheduler",
		"sudo.enable":                                  defaultEnabled,
		"support.hive.oneSession":                      defaultDisabled,
		"task.resource.limit.state":                    defaultDisabled,
		"yarn.application.status.address":              "http://ds1:%s/ws/v1/cluster/apps/%s",
		"yarn.job.history.status.address":              "http://ds1:19888/ws/v1/history/mapreduce/jobs/%s",
		"yarn.resourcemanager.ha.rm.ids":               "192.168.xx.xx,192.168.xx.xx",
	}
}

// defaultEnvOverrides are the product default environment values. They also seed the
// "<cluster>-envs" ConfigMap the containers envFrom (see ClusterExtension).
func defaultEnvOverrides() map[string]string {
	return map[string]string{
		"TZ":                       "Asia/Shanghai",
		"SPRING_JACKSON_TIME_ZONE": "Asia/Shanghai",
	}
}

// ResolveRoleGroup is the GenericReconcilerConfig.RoleGroupResolver hook: the product's
// configuration contribution, folded as the LOWEST layer beneath role and role-group overrides,
// so user overrides always win.
func ResolveRoleGroup(
	_ context.Context,
	_ ctrlclient.Client,
	_ *dolphinv1alpha1.DolphinschedulerCluster,
	_ *reconciler.RoleGroupBuildContext,
) (*reconciler.Contribution, error) {
	return &reconciler.Contribution{
		ConfigOverrides: map[string]map[string]string{
			dolphinv1alpha1.DolphinCommonPropertiesName: defaultCommonProperties(),
		},
		EnvVars: defaultEnvOverrides(),
	}, nil
}

// roleEnvDefaults returns the role-specific default container environment as a SORTED
// corev1.EnvVar list, byte-identical (names, values, order) to the legacy rendering.
func roleEnvDefaults(roleName string) []corev1.EnvVar {
	var env map[string]string
	switch roleName {
	case dolphinv1alpha1.RoleMaster:
		env = map[string]string{
			envJavaOpts:                                    "-Xms512m -Xmx512m -Xmn256m",
			"MASTER_DISPATCH_TASK_NUM":                     "3",
			"MASTER_EXEC_TASK_NUM":                         "20",
			"MASTER_EXEC_THREADS":                          "100",
			"MASTER_FAILOVER_INTERVAL":                     "10m",
			"MASTER_HEARTBEAT_ERROR_THRESHOLD":             "5",
			"MASTER_HOST_SELECTOR":                         "LowerWeight",
			"MASTER_KILL_APPLICATION_WHEN_HANDLE_FAILOVER": "true",
			"MASTER_MAX_HEARTBEAT_INTERVAL":                "10s",
			"MASTER_SERVER_LOAD_PROTECTION_ENABLED":        "false",
			"MASTER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS":          defaultLoadProtectionThreshold,
			"MASTER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS":       defaultLoadProtectionThreshold,
			"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS":    defaultLoadProtectionThreshold,
			"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS": defaultLoadProtectionThreshold,
			"MASTER_STATE_WHEEL_INTERVAL":                                                 "5s",
			"MASTER_TASK_COMMIT_INTERVAL":                                                 "1s",
			"MASTER_TASK_COMMIT_RETRYTIMES":                                               "5",
		}
	case dolphinv1alpha1.RoleWorker:
		env = map[string]string{
			"DEFAULT_TENANT_ENABLED":                defaultDisabled,
			"WORKER_EXEC_THREADS":                   "100",
			"WORKER_HOST_WEIGHT":                    "100",
			"WORKER_MAX_HEARTBEAT_INTERVAL":         "10s",
			"WORKER_SERVER_LOAD_PROTECTION_ENABLED": defaultDisabled,
			"WORKER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS":          defaultLoadProtectionThreshold,
			"WORKER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS":       defaultLoadProtectionThreshold,
			"WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS":    defaultLoadProtectionThreshold,
			"WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS": defaultLoadProtectionThreshold,
			"WORKER_TENANT_CONFIG_AUTO_CREATE_TENANT_ENABLED":                             "true",
			"WORKER_TENANT_CONFIG_DISTRIBUTED_TENANT":                                     "false",
		}
	case dolphinv1alpha1.RoleApi, dolphinv1alpha1.RoleAlert:
		env = map[string]string{
			envJavaOpts: "-Xms512m -Xmx512m -Xmn256m",
		}
	default:
		return nil
	}

	vars := make([]corev1.EnvVar, 0, len(env))
	for _, k := range slices.Sorted(maps.Keys(env)) {
		vars = append(vars, corev1.EnvVar{Name: k, Value: env[k]})
	}
	return vars
}

// roleResourceDefaults returns the legacy per-role CPU/memory/storage defaults.
func roleResourceDefaults(roleName string) (*commonsv1alpha1.ResourcesSpec, error) {
	var cpuMin, cpuMax, memoryLimit, storage string
	switch roleName {
	case dolphinv1alpha1.RoleMaster:
		cpuMin, cpuMax, memoryLimit, storage = "300m", "500m", "800Mi", quantity1Gi
	case dolphinv1alpha1.RoleWorker:
		cpuMin, cpuMax, memoryLimit, storage = quantity400m, "600m", quantity1Gi, "2Gi"
	case dolphinv1alpha1.RoleApi:
		cpuMin, cpuMax, memoryLimit, storage = quantity400m, "700m", quantity1Gi, quantity1Gi
	case dolphinv1alpha1.RoleAlert:
		cpuMin, cpuMax, memoryLimit, storage = "300m", quantity400m, "800Mi", quantity1Gi
	default:
		return nil, fmt.Errorf("unknown role %q for resource defaults", roleName)
	}
	return &commonsv1alpha1.ResourcesSpec{
		CPU: &commonsv1alpha1.CPUResource{
			Min: ptr.To(resource.MustParse(cpuMin)),
			Max: ptr.To(resource.MustParse(cpuMax)),
		},
		Memory: &commonsv1alpha1.MemoryResource{
			Limit: ptr.To(resource.MustParse(memoryLimit)),
		},
		Storage: &commonsv1alpha1.StorageResource{
			Capacity: ptr.To(resource.MustParse(storage)),
		},
	}, nil
}

// defaultRoleAffinity returns the legacy product-default preferred pod anti-affinity
// (weight 70, hostname topology, matching the role's instance+component labels).
// reconciler.RoleSelectorLabels emits exactly the app.kubernetes.io/{instance,component} pair
// the legacy literal used, so the rendered affinity is byte-identical.
func defaultRoleAffinity(clusterName, roleName string) (*runtime.RawExtension, error) {
	return reconciler.EncodeAffinity(&corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				reconciler.PreferredAffinityTerm(70, reconciler.TopologyKeyHostname,
					reconciler.RoleSelectorLabels(clusterName, roleName)),
			},
		},
	})
}

// metricsPortForRole returns the Spring actuator port of a role: the port the probes hit and
// the metrics Service exposes.
func metricsPortForRole(roleName string) (int32, error) {
	switch roleName {
	case dolphinv1alpha1.RoleMaster:
		return dolphinv1alpha1.MasterActualPort, nil
	case dolphinv1alpha1.RoleWorker:
		return dolphinv1alpha1.WorkerActualPort, nil
	case dolphinv1alpha1.RoleApi:
		return dolphinv1alpha1.ApiPort, nil
	case dolphinv1alpha1.RoleAlert:
		return dolphinv1alpha1.AlerterActualPort, nil
	default:
		return 0, fmt.Errorf("unknown role %q for metrics port", roleName)
	}
}
