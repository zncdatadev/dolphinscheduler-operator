package master

import (
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Master), groupName).GenerateResourceName("")
}

func createSvcName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Master), groupName).GenerateResourceName("")
}

func configVolumeName() string {
	return "config"
}
