package reconcile

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	emptyResult = ctrl.Result{}
)

type ReconcileResult struct {
	result ctrl.Result
	err    error
	done   bool
}

func reconcileResult(result ctrl.Result, err error) ReconcileResult {
	return ReconcileResult{
		result: result,
		err:    err,
		done:   true,
	}
}

func DoNotRequeue() ReconcileResult {
	return reconcileResult(emptyResult, nil)
}

func RequeueOnError(err error) ReconcileResult {
	return reconcileResult(emptyResult, err)
}

func RequeueAfter(duration time.Duration) ReconcileResult {
	return reconcileResult(ctrl.Result{RequeueAfter: duration}, nil)
}

func (r ReconcileResult) IsDone() bool {
	return r.done
}

func (r ReconcileResult) Get() (ctrl.Result, error) {
	return r.result, r.err
}
