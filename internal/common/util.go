package common

import (
	"fmt"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/util"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

func OverrideEnvVars(origin *[]corev1.EnvVar, override map[string]string) {
	var originVars = make(map[string]int)
	for i, env := range *origin {
		originVars[env.Name] = i
	}

	for k, v := range override {
		// if env Name is in override, then override it
		if idx, ok := originVars[k]; ok {
			(*origin)[idx].Value = v
		} else {
			// if override's key is new, then append it
			*origin = append(*origin, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
}

func ConfigConfigMapName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, "", groupName).GenerateResourceName("config")
}

func EnvsConfigMapName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, "", groupName).GenerateResourceName("envs")
}

func CreateNetworkUrl(podName string, svcName, namespace, clusterDomain string, port int32) string {
	return podName + "." + CreateDnsDomain(svcName, namespace, clusterDomain, port)
}

func CreateDnsDomain(svcName, namespace, clusterDomain string, port int32) string {
	return fmt.Sprintf("%s:%d", CreateDomainHost(svcName, namespace, clusterDomain), port)
}

func CreateDomainHost(svcName, namespace, clusterDomain string) string {
	return fmt.Sprintf("%s.%s.svc.%s", svcName, namespace, clusterDomain)
}

// CreatePodNamesByReplicas create pod names by replicas
func CreatePodNamesByReplicas(replicas int32, statefulResourceName string) []string {
	podNames := make([]string, replicas)
	for i := int32(0); i < replicas; i++ {
		podName := fmt.Sprintf("%s-%d", statefulResourceName, i)
		podNames[i] = podName
	}
	return podNames
}

func CreateServiceAccountName(instanceName string) string {
	return util.NewResourceNameGeneratorOneRole(instanceName, "").GenerateResourceName("sa")
}

func CreateKvContentByReplicas(replicas int32, keyTemplate string, valueTemplate string) [][2]string {
	var res [][2]string
	for i := int32(0); i < replicas; i++ {
		key := fmt.Sprintf(keyTemplate, i)
		var value string
		if strings.Contains(valueTemplate, "%d") {
			value = fmt.Sprintf(valueTemplate, i)
		} else {
			value = valueTemplate
		}
		res = append(res, [2]string{key, value})
	}
	return res
}

const (
	DefaultFileAppender = "FILE"
	NoneAppender        = "None"
	ConsoleLogAppender  = "dolphinAppender"
	FileLogAppender     = NoneAppender
)

func CreateLog4jBuilder(containerLogging *dolphinv1alpha1.LoggingConfigSpec, consoleAppenderName,
	fileAppenderName string, fileLogLocation string) *Log4jLoggingDataBuilder {
	log4jBuilder := &Log4jLoggingDataBuilder{}
	if loggers := containerLogging.Loggers; loggers != nil {
		var builderLoggers []LogBuilderLoggers
		for logger, level := range loggers {
			builderLoggers = append(builderLoggers, LogBuilderLoggers{
				logger: logger,
				level:  level.Level,
			})
		}
		log4jBuilder.Loggers = builderLoggers
	}
	if console := containerLogging.Console; console != nil {
		log4jBuilder.Console = &LogBuilderAppender{
			appenderName: consoleAppenderName,
			level:        console.Level,
		}
	}
	if file := containerLogging.File; file != nil {
		log4jBuilder.File = &LogBuilderAppender{
			appenderName:       fileAppenderName,
			level:              file.Level,
			defaultLogLocation: fileLogLocation,
		}
	}

	return log4jBuilder
}

func K8sEnvRef(envName string) string {
	return fmt.Sprintf("$(%s)", envName)
}

func LinuxEnvRef(envName string) string {
	return fmt.Sprintf("$%s", envName)
}

func PdbCfg(pdbSpec *dolphinv1alpha1.PodDisruptionBudgetSpec) *PdbConfig {
	return &PdbConfig{
		MaxUnavailable: pdbSpec.MaxUnavailable,
		MinAvailable:   pdbSpec.MinAvailable,
	}
}
func ExtractDataBaseReference(dbSpec *dolphinv1alpha1.DatabaseSpec) (*DatabaseConfiguration, *DatabaseParams) {
	inlineDb := dbSpec.Inline
	db := DatabaseConfiguration{
		DbReference: &dbSpec.Reference,
		DbInline: NewDatabaseParams(
			inlineDb.Driver,
			inlineDb.Username,
			inlineDb.Password,
			inlineDb.Host,
			strconv.Itoa(int(inlineDb.Port)),
			inlineDb.DatabaseName),
	}
	params, err := db.GetDatabaseParams()
	if err != nil {
		panic(err)
	}
	return &db, params
}

func MakeDataBaseEnvs(dbSpec *dolphinv1alpha1.DatabaseSpec) []corev1.EnvVar {
	db, params := ExtractDataBaseReference(dbSpec)
	uri, err := db.GetURI()
	if err != nil {
		panic(err)
	}
	return []corev1.EnvVar{
		{
			Name:  "DATABASE",
			Value: string(params.DbType),
		},
		{
			Name:  "SPRING_DATASOURCE_URL",
			Value: uri,
		},
		{
			Name:  "SPRING_DATASOURCE_USERNAME",
			Value: params.Username,
		},
		{
			Name:  "SPRING_DATASOURCE_PASSWORD",
			Value: params.Password,
		},
		{
			Name:  "SPRING_DATASOURCE_DRIVER-CLASS-NAME",
			Value: params.Driver,
		},
	}
}

var _ RoleGroupConfigGetter = &GenericRoleGroupGetter{}

type GenericRoleGroupGetter struct {
	RoleConfigGetter
	mergedCfg *dolphinv1alpha1.RoleGroupSpec
}

func (r *GenericRoleGroupGetter) Replicas() int32 {
	return r.mergedCfg.Replicas
}

func NewRoleGroupTransformer(
	roleConfigGetter RoleConfigGetter,
	groupSpec *dolphinv1alpha1.RoleGroupSpec) *GenericRoleGroupGetter {
	return &GenericRoleGroupGetter{RoleConfigGetter: roleConfigGetter, mergedCfg: groupSpec}
}

func (r *GenericRoleGroupGetter) MergedConfig() any {
	return r.mergedCfg
}

func (r *GenericRoleGroupGetter) GroupPdbSpec() *PdbConfig {
	pdbSpec := r.mergedCfg.Config.PodDisruptionBudget
	if pdbSpec == nil {
		return nil
	}
	return &PdbConfig{
		MinAvailable:   pdbSpec.MinAvailable,
		MaxUnavailable: pdbSpec.MaxUnavailable,
	}
}

func (r *GenericRoleGroupGetter) NodeSelector() map[string]string {
	return r.mergedCfg.Config.NodeSelector
}

var _ InstanceAttributes = &DolphinSchedulerClusterInstance{}

type DolphinSchedulerClusterInstance struct {
	Instance *dolphinv1alpha1.DolphinschedulerCluster
}

func (k *DolphinSchedulerClusterInstance) GetRoleConfig(role Role) *RoleConfiguration {
	switch role {
	case Master:
		masterSpec := k.Instance.Spec.Master
		roleGetter := &DolphinSchedulerRoleGetter{masterSpec.Config}
		groups := maps.Keys(masterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, masterSpec.PodDisruptionBudget)
	case Worker:
		workerSpec := k.Instance.Spec.Worker
		roleGetter := &DolphinSchedulerRoleGetter{workerSpec.Config}
		groups := maps.Keys(workerSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, workerSpec.PodDisruptionBudget)
	case Alerter:
		alerterSpec := k.Instance.Spec.Alerter
		roleGetter := &DolphinSchedulerRoleGetter{alerterSpec.Config}
		groups := maps.Keys(alerterSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, alerterSpec.PodDisruptionBudget)
	case Api:
		apiSpec := k.Instance.Spec.Api
		roleGetter := &DolphinSchedulerRoleGetter{apiSpec.Config}
		groups := maps.Keys(apiSpec.RoleGroups)
		return k.transformRoleSpec(roleGetter, groups, apiSpec.PodDisruptionBudget)
	default:
		panic(fmt.Sprintf("unknown role: %s", role))
	}
}

// masterSpec to RoleConfiguration
func (k *DolphinSchedulerClusterInstance) transformRoleSpec(
	roleConfigGetter RoleConfigGetter,
	groups []string,
	rolePdbSpec *dolphinv1alpha1.PodDisruptionBudgetSpec,
) *RoleConfiguration {
	return NewRoleConfiguration(roleConfigGetter.Config(), groups,
		&PdbConfig{
			MinAvailable:   rolePdbSpec.MinAvailable,
			MaxUnavailable: rolePdbSpec.MinAvailable,
		})
}

func (k *DolphinSchedulerClusterInstance) GetClusterConfig() any {
	return k.Instance.Spec.ClusterConfigSpec
}

func (k *DolphinSchedulerClusterInstance) GetNamespace() string {
	return k.Instance.GetNamespace()
}

func (k *DolphinSchedulerClusterInstance) GetInstanceName() string {
	return k.Instance.GetName()
}

var _ RoleConfigGetter = &DolphinSchedulerRoleGetter{}

type DolphinSchedulerRoleGetter struct {
	roleSpec *dolphinv1alpha1.ConfigSpec
}

func (d DolphinSchedulerRoleGetter) Config() any {
	return d.roleSpec
}
