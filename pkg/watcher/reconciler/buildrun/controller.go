package buildrun

import (
	"context"
	"github.com/tektoncd/results/pkg/watcher/logs"
	"github.com/tektoncd/results/pkg/watcher/reconciler"
	pb "github.com/tektoncd/results/proto/v1alpha2/results_go_proto"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	knativereconciler "knative.dev/pkg/reconciler"
)

func NewController(ctx context.Context, resultsClient pb.ResultsClient, cmw configmap.Watcher) *controller.Impl {
	return NewControllerWithConfig(ctx, resultsClient, &reconciler.Config{}, cmw)
}

func NewControllerWithConfig(ctx context.Context, client pb.ResultsClient, r *reconciler.Config, cmw configmap.Watcher) *controller.Impl {
	informer := buildruninformer.Get(ctx)

	c := &Reconciler{
		LeaderAwareFuncs: knativereconciler.LeaderAwareFuncs{},
		resultsClient:    client,
		logsClient:       logs.Get(ctx),
		buildRunLister:   lister,
		shipwrightClient: ,
		cfg:              nil,
		configStore:      nil,
	}
}
