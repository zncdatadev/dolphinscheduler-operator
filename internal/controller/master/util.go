package master

import (
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(core.Master), groupName).GenerateResourceName("")
}

func createSvcName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(core.Master), groupName).GenerateResourceName("")
}

func configVolumeName() string {
	return "config"
}
