package master

import (
	"fmt"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/core"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(getRole()), groupName).GenerateResourceName("")
}

func createSvcName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(getRole()), groupName).GenerateResourceName("")
}

func configVolumeName() string {
	return "config"
}

func logbackConfigVolumeName() string {
	return "logback"
}

func logbackConfigMapName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(getRole()), groupName).GenerateResourceName("logback")
}

func logbackMountPath() string {
	return fmt.Sprintf("%s/%s", dolphinv1alpha1.DolphinConfigPath, dolphinv1alpha1.LogbackPropertiesFileName)
}

func getRole() core.Role {
	return common.Master
}
