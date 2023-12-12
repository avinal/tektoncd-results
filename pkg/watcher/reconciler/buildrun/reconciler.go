package buildrun

import (
	"context"
	"fmt"
	"github.com/shipwright-io/build/pkg/apis/build/v1beta1"
	"github.com/shipwright-io/build/pkg/client/clientset/versioned"
	brv1beta1 "github.com/shipwright-io/build/pkg/client/listers/build/v1beta1"
	"github.com/tektoncd/results/pkg/apis/config"
	"github.com/tektoncd/results/pkg/watcher/reconciler"
	"github.com/tektoncd/results/pkg/watcher/reconciler/dynamic"
	"github.com/tektoncd/results/pkg/watcher/results"
	pb "github.com/tektoncd/results/proto/v1alpha2/results_go_proto"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	knativereconciler "knative.dev/pkg/reconciler"
)

type Reconciler struct {
	knativereconciler.LeaderAwareFuncs

	resultsClient    pb.ResultsClient
	logsClient       pb.LogsClient
	buildRunLister   brv1beta1.BuildRunLister
	shipwrightClient versioned.Interface
	cfg              *reconciler.Config
	configStore      *config.Config
}

type BuildRunWrapper struct {
	*v1beta1.BuildRun
}

func (brw *BuildRunWrapper) GetStatusCondition() apis.ConditionAccessor {
	return &BuildRunStatusWrapper{&brw.Status}
}

type BuildRunStatusWrapper struct {
	*v1beta1.BuildRunStatus
}

func (brsw *BuildRunStatusWrapper) GetCondition(t apis.ConditionType) *apis.Condition {
	for _, c := range brsw.Conditions {
		if string(c.Type) == string(t) {
			return buildRunConditionToConditions(c)
		}
	}
	return nil
}

func buildRunConditionToConditions(c v1beta1.Condition) *apis.Condition {
	return &apis.Condition{
		Type:               apis.ConditionType(c.Type),
		Status:             c.Status,
		LastTransitionTime: apis.VolatileTime{Inner: c.LastTransitionTime},
		Reason:             c.Reason,
		Message:            c.Message,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx).With(zap.String("results.tekton.dev/kind", "BuildRun"))

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("Received invalid resource key '%s', skipping reconciliation.", key)
		return nil
	}

	if !r.IsLeaderFor(types.NamespacedName{Namespace: namespace, Name: name}) {
		logger.Debugf("Instance is not the leader for BuildRun '%s/%s', skipping reconciliation.", namespace, name)
		return controller.NewSkipKey(key)
	}

	logger.Infof("Initializing reconcilation for BuildRun '%s/%s'", namespace, name)

	br, err := r.buildRunLister.BuildRuns(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debugf("BuildRun '%s/%s' is no longer available, skipping reconcilation.", namespace, name)
			return controller.NewSkipKey(key)
		}
		return fmt.Errorf("error retrieving BuildRun '%s/%s' from indexer: %w", namespace, name, err)
	}

	buildRunClient := &dynamic.BuildRunClient{
		BuildRunInterface: r.shipwrightClient.ShipwrightV1beta1().BuildRuns(namespace),
	}

	brw := &BuildRunWrapper{br}

	dyn := dynamic.NewDynamicReconciler(r.resultsClient, r.logsClient, buildRunClient, r.cfg)
	dyn.AfterDeletion = func(ctx context.Context, object results.Object) error {
		return nil
	}
	return dyn.Reconcile(logging.WithLogger(ctx, logger), brw)
}
