package controller

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/mrp_controller/metrics"
	"github.com/argoproj/argo-cd/v3/mrp_controller/service"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type MRPController interface {
	Run(ctx context.Context)
}

type ControllerOpts struct {
	// EventTimeout is the context deadline for handling a single application.
	EventTimeout time.Duration
	// NumWorkers is the number of the goroutines handling the applications.
	NumWorkers int
	// Retry defines how an application whose handling failed is retried.
	Retry RetryPolicy
}

type monorepoController struct {
	appBroadcaster AppEventHandler
	acrService     service.MRPService
	appQueue       *KeyedAppQueue
	opts           ControllerOpts
}

func NewMonorepoController(appInformer cache.SharedIndexInformer, applicationClientset appclientset.Interface, db db.ArgoDB, repoClientset repoapiclient.Clientset, metricsServer *metrics.MetricsServer, opts ControllerOpts) MRPController {
	appBroadcaster := NewAppEventHandler()
	_, err := appInformer.AddEventHandler(appBroadcaster)
	if err != nil {
		log.Error(err)
	}
	appQueue := NewKeyedAppQueue(WithRetryPolicy(opts.Retry), WithRetryMetrics(metricsServer))
	appQueue.SetLengthGauge(metricsServer.QueueLengthGauge())
	return &monorepoController{
		appBroadcaster: appBroadcaster,
		acrService:     service.NewMRPService(applicationClientset, db, repoClientset, metricsServer),
		appQueue:       appQueue,
		opts:           opts,
	}
}

func (c *monorepoController) Run(ctx context.Context) {
	var logCtx log.FieldLogger = log.StandardLogger()

	// The error is reported to the queue, which logs it and decides whether
	// the application is handled again.
	calculateIfPermitted := func(ctx context.Context, a appv1.Application) error {
		logCtx.Debugf("calculateIfPermitted called for application '%s/%s'", a.Namespace, a.Name)
		ctx, cancel := context.WithTimeout(ctx, c.opts.EventTimeout)
		defer cancel()
		return c.acrService.ChangeRevision(ctx, &a)
	}

	enqueueFunc := func(event *appv1.ApplicationWatchEvent) error {
		c.appQueue.Enqueue(event.Application)
		return nil
	}
	filterFunc := func(event *appv1.ApplicationWatchEvent) bool {
		if event.Type == watch.Bookmark || event.Type == watch.Deleted {
			return false
		}
		val, ok := event.Application.Annotations[appv1.AnnotationKeyManifestGeneratePaths]
		if !ok || val == "" {
			return false
		}
		return true
	}

	// No sense to retry a deleted application
	forgetFunc := func(event *appv1.ApplicationWatchEvent) error {
		c.appQueue.ForgetApp(AppKey(&event.Application))
		return nil
	}
	deletedFilterFunc := func(event *appv1.ApplicationWatchEvent) bool {
		return event.Type == watch.Deleted
	}

	unsubscribe := c.appBroadcaster.Subscribe(enqueueFunc, filterFunc)
	unsubscribeDeleted := c.appBroadcaster.Subscribe(forgetFunc, deletedFilterFunc)

	wp := NewWorkerPool(c.appQueue, c.opts.NumWorkers, calculateIfPermitted)

	defer func() {
		unsubscribe()
		unsubscribeDeleted()
		wp.Stop()
	}()
	<-ctx.Done()
	logCtx.Debug("got Done event")
}
