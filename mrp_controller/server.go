package mrp_controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/errors"

	mrp_controller "github.com/argoproj/argo-cd/v3/mrp_controller/controller"
	"github.com/argoproj/argo-cd/v3/mrp_controller/metrics"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
	applister "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/glob"

	// applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"

	settings_util "github.com/argoproj/argo-cd/v3/util/settings"
)

type MRPServer struct {
	MRPServerOpts

	settings *settings_util.ArgoCDSettings
	// log                  *log.Entry
	appInformer          cache.SharedIndexInformer
	appLister            applister.ApplicationLister
	applicationClientset appclientset.Interface
	db                   db.ArgoDB
	repoClientset        repoapiclient.Clientset
	// stopCh is the channel which when closed, will shutdown the Event Reporter server
	stopCh        chan struct{}
	metricsServer *metrics.MetricsServer

	// serviceSet *MRPServerSet
}

type MRPServerSet struct{}

type MRPServerOpts struct {
	ListenPort    int
	ListenHost    string
	Namespace     string
	KubeClientset kubernetes.Interface
	AppClientset  appclientset.Interface
	//Cache                 *servercache.Cache
	RedisClient            *redis.Client
	ApplicationNamespaces  []string
	BaseHRef               string
	RootPath               string
	RepoClientset          repoapiclient.Clientset
	MetricsCacheExpiration time.Duration
}

type handlerSwitcher struct {
	handler              http.Handler
	urlToHandler         map[string]http.Handler
	contentTypeToHandler map[string]http.Handler
}

type Listeners struct {
	Main net.Listener
}

func (l *Listeners) Close() error {
	if l.Main != nil {
		if err := l.Main.Close(); err != nil {
			return err
		}
		l.Main = nil
	}
	return nil
}

func (a *MRPServer) healthCheck(_ *http.Request) error {
	return nil
}

// Init starts informers used by the API server
func (a *MRPServer) Init(ctx context.Context) {
	go a.appInformer.Run(ctx.Done())
	// svcSet := newApplicationChangeRevisionServiceSet()
	// a.serviceSet = svcSet
}

func (a *MRPServer) RunController(ctx context.Context) {
	controller := mrp_controller.NewMonorepoController(a.appInformer, a.applicationClientset, a.db, a.repoClientset, a.metricsServer)
	go controller.Run(ctx)
}

func (a *MRPServer) checkServeErr(name string, err error) {
	if err != nil {
		if a.stopCh != nil {
			log.Fatalf("%s: %v", name, err)
		}
		// a nil stopCh indicates a graceful shutdown
		log.Infof("graceful shutdown %s: %v", name, err)
	} else {
		log.Infof("graceful shutdown %s", name)
	}
}

// isAppNamespaceAllowed returns whether the application is allowed in the
// namespace it's residing in.
func (a *MRPServer) isAppNamespaceAllowed(app *appv1.Application) bool {
	return app.Namespace == a.Namespace ||
		glob.MatchStringInList(a.ApplicationNamespaces, app.Namespace, glob.REGEXP)
}

func (a *MRPServer) canProcessApp(obj any) bool {
	return true
	app, ok := obj.(*appv1.Application)
	if !ok {
		return false
	}

	if !a.isAppNamespaceAllowed(app) {
		return false
	}

	annotations := app.GetAnnotations()
	if annotations == nil {
		return false
	}
	val, ok := annotations[appv1.AnnotationKeyManifestGeneratePaths]
	if !ok || val == "" {
		return false
	}
	return true
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *MRPServer) Run(ctx context.Context /* lns *Listeners*/) {
	// httpS := a.newHTTPServer(ctx, a.ListenPort)
	// tlsConfig := tls.Config{}
	// tlsConfig.GetCertificate = func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// 	return a.settings.Certificate, nil
	// }
	var appConditions []string
	var appLabels []string
	healthcheck := func(_ *http.Request) error { return nil }

	metricsAddr := fmt.Sprintf("%s:%d", a.ListenHost, a.ListenPort)
	metricsServer, err := metrics.NewMetricsServer(metricsAddr,
		a.appLister,     /*AppLister*/
		a.canProcessApp, /*AppFilter*/
		healthcheck,     /*healthCheck */
		appLabels,       /* appLabels */
		appConditions,   /* appCondition */
		a.db,
	)
	if err != nil {
		log.Fatal("Failed to configure metrics server: %w", err)
	}
	if a.MetricsCacheExpiration.Seconds() > 0 {
		metricsServer.SetExpiration(a.MetricsCacheExpiration)
	}
	a.metricsServer = metricsServer

	go func() { errors.CheckError(a.metricsServer.ListenAndServe()) }()
	go a.RunController(ctx)

	if !cache.WaitForCacheSync(ctx.Done(), a.appInformer.HasSynced) {
		log.Fatal("Timed out waiting for project cache to sync")
	}

	a.stopCh = make(chan struct{})
	<-a.stopCh
}

// Returns a new instance of the Monorepo Controller Server
func NewMRPServer(ctx context.Context, opts MRPServerOpts) *MRPServer {
	appInformerNs := opts.Namespace
	if len(opts.ApplicationNamespaces) > 0 {
		appInformerNs = ""
	}
	appFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(appInformerNs), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))

	appInformer := appFactory.Argoproj().V1alpha1().Applications().Informer()
	appLister := appFactory.Argoproj().V1alpha1().Applications().Lister()

	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	dbInstance := db.NewDB(opts.Namespace, settingsMgr, opts.KubeClientset)
	// repoclientset := repoapiclient.NewRepoServerClientset(repoServerAddress,
	// 	repoServerTimeoutSeconds,
	// 	tlsConfig)

	server := &MRPServer{
		MRPServerOpts: opts,
		// log:                  log.NewEntry(log.StandardLogger()),
		appInformer:          appInformer,
		appLister:            appLister,
		applicationClientset: opts.AppClientset,
		db:                   dbInstance,
		repoClientset:        opts.RepoClientset,
		//metricsServer:        metricsServer,
	}

	return server
}
