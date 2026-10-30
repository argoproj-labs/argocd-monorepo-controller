package commands

import (
	"context"
	"fmt"
	"math"
	"time"

	mrp "github.com/argoproj/argo-cd/v3/mrp_controller"
	mrpcontroller "github.com/argoproj/argo-cd/v3/mrp_controller/controller"

	"github.com/argoproj/pkg/stats"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/kube"
)

var (
	failureRetryCount              = 0
	failureRetryPeriodMilliSeconds = 100
)

// minAppResyncPeriod is the shortest resync period accepted.
const minAppResyncPeriod = time.Minute

// func init() {
// 	failureRetryCount = env.ParseNumFromEnv(failureRetryCountEnv, failureRetryCount, 0, 10)
// 	failureRetryPeriodMilliSeconds = env.ParseNumFromEnv(failureRetryPeriodMilliSecondsEnv, failureRetryPeriodMilliSeconds, 0, 1000)
// }

// NewCommand returns a new instance of an application change revision command
func NewCommand() *cobra.Command {
	var (
		redisClient  *redis.Client
		listenHost   string
		listenPort   int
		glogLevel    int
		clientConfig clientcmd.ClientConfig
		// applicationServerAddress string
		repoServerAddress        string
		repoServerTimeoutSeconds int
		repoServerPlaintext      bool
		repoServerStrictTLS      bool
		// cacheSrc                 func() (*servercache.Cache, error)
		applicationNamespaces       []string
		metricsCacheExpiration      time.Duration
		eventHandlingTimeoutSeconds int
		controllerWorkers           int
		retryPolicy                 mrpcontroller.RetryPolicy
		appResyncPeriod             time.Duration
		// argocdToken              string
		// rootpath                 string
	)
	command := &cobra.Command{
		Use:               cliName,
		Short:             "Run the ArgoCD Monorepo Controller",
		Long:              "The ArgoCD Monorepo Controller is a service that listens for application events and updates the application's revision in the application CRD",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, _ []string) {
			ctx := c.Context()

			vers := common.GetVersion()

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD Monorepo Controller",
				map[string]any{
					"namespace": namespace,
					"port":      listenPort,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(glogLevel)

			errors.CheckError(retryPolicy.Validate())
			if appResyncPeriod != 0 && appResyncPeriod < minAppResyncPeriod {
				log.Fatalf("invalid application resync period %v: must be 0 to disable the resync, or at least %v", appResyncPeriod, minAppResyncPeriod)
			}

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			// cache, err := cacheSrc()
			// errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)

			appclientsetConfig, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(appclientsetConfig))
			config.UserAgent = fmt.Sprintf("argocd-server/%s (%s)", vers.Version, vers.Platform)

			if failureRetryCount > 0 {
				appclientsetConfig = kube.AddFailureRetryWrapper(appclientsetConfig, failureRetryCount, failureRetryPeriodMilliSeconds)
			}
			appClientSet := appclientset.NewForConfigOrDie(appclientsetConfig)

			tlsConfig := repoapiclient.TLSConfiguration{
				DisableTLS:       repoServerPlaintext,
				StrictValidation: repoServerStrictTLS,
			}
			repoClientSet := repoapiclient.NewRepoServerClientset(repoServerAddress,
				repoServerTimeoutSeconds,
				tlsConfig)

			changeRevisionServerOpts := mrp.MRPServerOpts{
				ListenPort:    listenPort,
				ListenHost:    listenHost,
				Namespace:     namespace,
				KubeClientset: kubeclientset,
				AppClientset:  appClientSet,
				// Cache:                 cache,
				RedisClient:            redisClient,
				ApplicationNamespaces:  applicationNamespaces,
				RepoClientset:          repoClientSet,
				MetricsCacheExpiration: metricsCacheExpiration,
				EventHandlingTimeout:   eventHandlingTimeoutSeconds,
				NumWorkers:             controllerWorkers,
				RetryPolicy:            retryPolicy,
				AppResyncPeriod:        appResyncPeriod,
			}

			log.Debug("Starting Monorepo Controller server")
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")
			changeRevisionServer := mrp.NewMRPServer(ctx, changeRevisionServerOpts)
			log.Debug("Initializing Monorepo Controller server")
			changeRevisionServer.Init(ctx)
			for {
				log.Debug("Running Monorepo controllerserver")
				ctx, cancel := context.WithCancel(ctx)
				changeRevisionServer.Run(ctx)
				log.Debug("Change revision controller server finished")
				cancel()
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("MONOREPO_CONTROLLER_LOGFORMAT", "json"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("MONOREPO_CONTROLLER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.AddCommand(cli.NewVersionCmd(cliName))
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("MONOREPO_CONTROLLER_LISTEN_ADDRESS", common.DefaultAddressMRPControllerMetrics), "Metrics server will listen on given address")
	command.Flags().IntVar(&listenPort, "port", env.ParseNumFromEnv("MONOREPO_CONTROLLER_LISTEN_PORT", common.DefaultPortMRPServerMetrics, 1, math.MaxInt16), "Metrics server will listen on given port")
	command.Flags().StringSliceVar(&applicationNamespaces, "application-namespaces", env.StringsFromEnv("MONOREPO_CONTROLLER_APPLICATION_NAMESPACES", []string{}, ","), "Comma separated list of additional namespaces where application resources can be managed in")
	command.Flags().StringVar(&repoServerAddress, "monorepo-repo-server", env.StringFromEnv("MONOREPO_REPO_SERVER", common.DefaultMonorepoRepoServerAddr), "Monorepo Repo server address")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "monorepo-repo-server-timeout-seconds", env.ParseNumFromEnv("MONOREPO_REPO_SERVER_TIMEOUT_SECONDS", 120, 0, math.MaxInt64), "Repo server RPC call timeout seconds.")
	command.Flags().BoolVar(&repoServerPlaintext, "monorepo-repo-server-plaintext", env.ParseBoolFromEnv("MONOREPO_REPO_SERVER_PLAINTEXT", false), "Use a plaintext client (non-TLS) to connect to repository server")
	command.Flags().BoolVar(&repoServerStrictTLS, "monorepo-repo-server-strict-tls", env.ParseBoolFromEnv("MONOREPO_REPO_SERVER_STRICT_TLS", false), "Perform strict validation of TLS certificates when connecting to monorepo repo server")
	command.Flags().DurationVar(&metricsCacheExpiration, "metrics-cache-expiration", env.ParseDurationFromEnv("MONOREPO_CONTROLLER_METRICS_CACHE_EXPIRATION", 0*time.Second, 0, math.MaxInt64), "Prometheus metrics cache expiration (disabled  by default. e.g. 24h0m0s)")
	command.Flags().IntVar(&eventHandlingTimeoutSeconds, "event-handling-timeout-seconds", env.ParseNumFromEnv("MONOREPO_CONTROLLER_EVENT_HANDLING_TIMEOUT_SECONDS", 120, 0, math.MaxInt64), "Context deadline for processing a single application event")
	command.Flags().IntVar(&controllerWorkers, "controller-workers", env.ParseNumFromEnv("MONOREPO_CONTROLLER_WORKERS", common.DefaultMonorepoControllerWorkers, 1, math.MaxInt32), "Number of worker goroutines processing application change-revision events")
	command.Flags().DurationVar(&appResyncPeriod, "app-resync-period", env.ParseDurationFromEnv("MONOREPO_CONTROLLER_APP_RESYNC_PERIOD", 0, 0, math.MaxInt64), "Interval at which every application is handled in absence of events. Disabled when it is 0s (e.g. 1h)")
	command.Flags().IntVar(&retryPolicy.MaxAttempts, "retry-max-attempts", env.ParseNumFromEnv("MONOREPO_CONTROLLER_RETRY_MAX_ATTEMPTS", mrpcontroller.DefaultRetryMaxAttempts, 0, math.MaxInt32), "Number of times handling of an application is retried after a transient failure. Retries are disabled when it is 0")
	command.Flags().DurationVar(&retryPolicy.BaseDelay, "retry-base-delay", env.ParseDurationFromEnv("MONOREPO_CONTROLLER_RETRY_BASE_DELAY", mrpcontroller.DefaultRetryBaseDelay, time.Millisecond, math.MaxInt64), "Delay before the first retry of a failed application (e.g. 10s)")
	command.Flags().DurationVar(&retryPolicy.MaxDelay, "retry-max-delay", env.ParseDurationFromEnv("MONOREPO_CONTROLLER_RETRY_MAX_DELAY", mrpcontroller.DefaultRetryMaxDelay, 0, math.MaxInt64), "Upper bound of the delay between the retries of a failed application. The delay is not capped when it is 0 (e.g. 5m)")
	command.Flags().Float64Var(&retryPolicy.Factor, "retry-backoff-factor", env.ParseFloat64FromEnv("MONOREPO_CONTROLLER_RETRY_BACKOFF_FACTOR", mrpcontroller.DefaultRetryBackoffFactor, 1, math.MaxFloat64), "Multiplier applied to the retry delay after every attempt")
	command.Flags().Float64Var(&retryPolicy.Jitter, "retry-jitter", env.ParseFloat64FromEnv("MONOREPO_CONTROLLER_RETRY_JITTER", mrpcontroller.DefaultRetryJitter, 0, 1), "Fraction by which the retry delay is randomized, so that the applications which failed together are not all retried at once")

	// cacheSrc = servercache.AddCacheFlagsToCmd(command, cacheutil.Options{
	// 	OnClientCreated: func(client *redis.Client) {
	// 		redisClient = client
	// 	},
	// })
	return command
}
