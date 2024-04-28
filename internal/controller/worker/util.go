package worker

import (
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
)

func createStatefulSetName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(core.Worker), groupName).GenerateResourceName("")
}

func createSvcName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(core.Worker), groupName).GenerateResourceName("")
}

func configVolumeName() string {
	return "config"
}

func workerDataVolumeName() string {
	return "data"
}
