package common

import (
	"fmt"
	"math"
	"path"
	"strings"

	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/constants"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
)

const Master util.Role = "master"
const Worker util.Role = "worker"
const Alerter util.Role = "alert"
const Api util.Role = "api"

func ClusterServiceName(instanceName string) string {
	return instanceName
}

func StatefulsetName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName()
}

func DeploymentName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName()
}

func RoleGroupConfigMapName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName()
}

func RoleGroupEnvsConfigMapName(instanceName string) string {
	return instanceName + "-envs"
}

func RoleGroupServiceName(roleGroupInfo *reconciler.RoleGroupInfo) string {
	return roleGroupInfo.GetFullName()
}

func ServiceAccountName(instanceName string) string {
	return builder.ServiceAccountName(instanceName)
}

func RoleName(instanceName string) string {
	return builder.RoleName(instanceName)
}

func RoleBindName(instanceName string) string {
	return builder.RoleBindingName(instanceName)
}

func PodFQDN(podName, svcName, namespace string) string {
	return fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, svcName, namespace)
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

func RoleBinBaseDir(role util.Role) string {
	// return fmt.Sprintf("%s/dolphinscheduler/", constants.KubedoopRoot)
	return path.Join(constants.KubedoopRoot, "dolphinscheduler")
}

func RoleServer(role util.Role) string {
	return fmt.Sprintf("%s-server", string(role))
}

func RoleConfigPath(role util.Role, configFileName string) string {
	return path.Join(RoleBinBaseDir(role), RoleServer(role), "conf", configFileName)
}

func RoleExecArgs(role string) string {
	return fmt.Sprintf("%s/bin/start.sh &", RoleServer(util.Role(role)))
}

func FixExecPermission(role string) string {
	return fmt.Sprintf("chmod +x %s/bin/start.sh", RoleServer(util.Role(role)))
}

func ToContainerPortInt32(n interface{}) (int32Port int32, err error) {
	switch v := n.(type) {
	case int:
		return int32(v), nil
	case int8:
		return int32(v), nil
	case int16:
		return int32(v), nil
	case int32:
		return v, nil
	case int64:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, fmt.Errorf("int64 value %d is out of int32 range", v)
		}
		return int32(v), nil
	default:
		return 0, fmt.Errorf("unexpected type %T to convert to int32", v)
	}
}
