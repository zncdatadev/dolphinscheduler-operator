package alerter

import (
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/util"
)

func createDeploymentName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Api), groupName).GenerateResourceName("")
}

func createSvcName(instanceName string, groupName string) string {
	return util.NewResourceNameGenerator(instanceName, string(common.Api), groupName).GenerateResourceName("")
}

func configVolumeName() string {
	return "config"
}
