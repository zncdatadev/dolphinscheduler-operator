package worker

import (
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

const (
	Role              = common.Worker
	MainContainerName = util.ContainerComponent("worker-server")
)
