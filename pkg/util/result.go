package util

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

func RequeueOrError(res ctrl.Result, err error) bool {
	return !res.IsZero() || err != nil
}
