package api

import (
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

const (
	Role              = common.Api
	MainContainerName = util.ContainerComponent("api-server")
)
