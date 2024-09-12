package master

import (
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

const (
	Role              = common.Master
	MainContainerName = util.ContainerComponent("master-server")
)
