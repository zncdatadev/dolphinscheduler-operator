package alerter

import (
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

const (
	Role              = common.Alerter
	MainContainerName = util.ContainerComponent("alerter-server")
)
