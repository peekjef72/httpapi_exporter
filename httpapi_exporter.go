package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

const (
	// Constant values
	metricsPublishingPort = ":9321"
	exporter_name         = "httpapi_exporter"
)

var pat_var_finder *regexp.Regexp

func init() {
	// initialize a global variable to detect and extract variable in format ${var}
	pat_var_finder, _ = regexp.Compile("{([^{]+)}")
}

var (
	// listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(metricsPublishingPort).String()
	metricsPath = kingpin.Flag("web.telemetry-path", "Path under which to expose collector's internal metrics.").Default("/metrics").String()
	configFile  = kingpin.Flag("config.file", "Exporter configuration file.").Short('c').Default("config/config.yml").String()
	//debug_flag    = kingpin.Flag("debug", "debug connection checks.").Short('d').Default("false").Bool()
	dry_run = kingpin.Flag("dry-run", "Only check exporter configuration file and exit.").Short('n').Default("false").Bool()
	// alsologtostderr = kingpin.Flag("alsologtostderr", "log to standard error as well as files.").Default("true").Bool()
	target_name    = kingpin.Flag("target", "In dry-run mode specify the target name, else ignored.").Short('t').String()
	model_name     = kingpin.Flag("model", "In dry-run mode specify the model name to build the dynamic target, else ignored.").Default("default").Short('m').String()
	auth_key       = kingpin.Flag("auth.key", "In dry-run mode specify the auth_key to use, else ignored.").Short('a').String()
	collector_name = kingpin.Flag("collector", "Specify the collector name restriction to collect, replace the collector_names set for each target.").Short('o').String()
	toolkitFlags   = kingpinflag.AddFlags(kingpin.CommandLine, metricsPublishingPort)
	logConfig      = promslog.Config{Style: promslog.GoKitStyle}
)

const (
	OpEgals = 1
	OpMatch = 2
)

type route struct {
	path    string
	regex   *regexp.Regexp
	handler http.HandlerFunc
}

type ctxKey struct {
}
type ctxValue struct {
	path string
}

func newRoute(op int, path string, handler http.HandlerFunc) *route {
	if op == OpEgals {
		return &route{path, nil, handler}
	} else if op == OpMatch {
		return &route{"", regexp.MustCompile("^" + path + "$"), handler}

	} else {
		return nil
	}

}
func BuildHandler(exporter Exporter, actionCh chan<- actionMsg) http.Handler {
	var routes = []*route{
		newRoute(OpEgals, "/", HomeHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/config", ConfigHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/health", HealthHandlerfunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/httpapi_exporter_metrics", func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) }),
		newRoute(OpEgals, "/reload", ReloadHandlerFunc(*metricsPath, exporter, actionCh)),
		newRoute(OpMatch, "/loglevel(?:/(.*))?", LogLevelHandlerFunc(*metricsPath, exporter, actionCh, "")),
		newRoute(OpEgals, "/status", StatusHandlerFunc(*metricsPath, exporter)),
		newRoute(OpMatch, "/targets(?:/(.*))?", TargetsHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, *metricsPath, func(w http.ResponseWriter, r *http.Request) { ExporterHandlerFor(exporter).ServeHTTP(w, r) }),
		// Expose exporter metrics separately, for debugging purposes.

		// pprof handle
		newRoute(OpMatch, "/debug/.+", http.DefaultServeMux.ServeHTTP),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, route := range routes {
			if route == nil {
				continue
			}
			if route.regex != nil {
				matches := route.regex.FindStringSubmatch(req.URL.Path)
				if len(matches) > 0 {
					var path string
					if len(matches) > 1 {
						path = matches[1]
					}

					ctxval := &ctxValue{
						path: path,
					}
					ctx := context.WithValue(req.Context(), ctxKey{}, ctxval)
					route.handler(w, req.WithContext(ctx))
					return
				}
			} else if req.URL.Path == route.path {
				route.handler(w, req)
				return
			}
		}
		err := errors.New("not found")
		HandleError(http.StatusNotFound, err, *metricsPath, exporter, w, req)
	})
}

type actionMsg struct {
	actiontype int
	logLevel   string
	retCh      chan error
}

const (
	ACTION_RELOAD   = iota
	ACTION_LOGLEVEL = iota
)

func main() {

	flag.AddFlags(kingpin.CommandLine, &logConfig)
	kingpin.Version(version.Print(exporter_name)).VersionFlag.Short('V')
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(&logConfig)
	logger.Info(fmt.Sprintf("Starting %s", exporter_name), "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())

	exporter, err := NewExporter(*configFile, logger, *collector_name)
	if err != nil {
		logger.Error(fmt.Sprintf("Error creating exporter: %s", err))
		os.Exit(1)
	}

	if exporter.Config().Globals.LogLevel != "" {
		logConfig.Level.Set(exporter.Config().Globals.LogLevel)
		logger = promslog.New(&logConfig)
	}
	exporter.SetLogLevel(logConfig.Level.String())

	if *dry_run {
		logger.Info("configuration OK.")
		// get the target if defined
		var (
			err   error
			t     Target
			tmp_t *TargetConfig
		)
		if *target_name != "" {
			*target_name = strings.TrimSpace(*target_name)
			t, err = exporter.FindTarget(*target_name)
			if err == ErrTargetNotFound {
				err = nil
				if *model_name != "" {
					*model_name = strings.TrimSpace(*model_name)
					t_def, err := exporter.FindTarget(*model_name)
					if err != nil {
						err := fmt.Errorf("Target model '%s' not found: %s", *model_name, err)
						logger.Error(err.Error())
						os.Exit(1)
					}
					if tmp_t, err = t_def.Config().Clone(*target_name, ""); err != nil {
						err := fmt.Errorf("invalid url set for remote_target '%s' %s", *target_name, err)
						logger.Error(err.Error())
						os.Exit(1)
					}
					t, err = exporter.AddTarget(tmp_t)
					if err != nil {
						err := fmt.Errorf("unable to create temporary target %s", err)
						logger.Error(err.Error())
						os.Exit(1)
					}
					exporter.Config().Targets = append(exporter.Config().Targets, tmp_t)
				}
			}
			if err == ErrTargetNotFound {
				logger.Warn(fmt.Sprintf("specified target %s not found. look for first available target", t.Name()))
				t, err = exporter.GetFirstTarget()
			}
			if err != nil {
				logger.Error(err.Error())
				os.Exit(1)
			}
			if *auth_key != "" {
				t.SetSymbol("auth_key", *auth_key)
			}

			logger.Info(fmt.Sprintf("try to collect target %s.", t.Name()))
			timeout := time.Duration(0)
			configTimeout := time.Duration(exporter.Config().Globals.ScrapeTimeout)

			// If the configured scrape timeout is more restrictive, use that instead.
			if configTimeout > 0 && (timeout <= 0 || configTimeout < timeout) {
				timeout = configTimeout
			}
			var ctx context.Context
			var cancel context.CancelFunc
			if timeout <= 0 {
				ctx = context.Background()
				cancel = func() {}
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), timeout)
			}
			defer cancel()

			gatherer := prometheus.Gatherers{exporter.WithContext(ctx, t, false)}
			mfs, err := gatherer.Gather()
			if err != nil {
				logger.Error(fmt.Sprintf("Error gathering metrics: %v", err))
				if len(mfs) == 0 {
					os.Exit(1)
				}
			} else {
				logger.Info("collect is OK. Dumping result to stdout.")
			}

			//dump metric to stdout
			enc := expfmt.NewEncoder(os.Stdout, `text/plain; version=`+expfmt.TextVersion+`; charset=utf-8`)

			for _, mf := range mfs {
				err := enc.Encode(mf)
				if err != nil {
					logger.Error(err.Error())
					break
				}
			}
			if closer, ok := enc.(expfmt.Closer); ok {
				// This in particular takes care of the final "# EOF\n" line for OpenMetrics.
				closer.Close()
			}
		}
		logger.Info("dry-run is over. Exiting.")
		os.Exit(0)
	}

	exporter.SetStartTime(time.Now())
	exporter.SetReloadTime(time.Now())

	user2 := make(chan os.Signal, 1)
	init_sigusr2(user2)
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	actionCh := make(chan actionMsg)
	go func() {
		for {
			select {
			case <-user2:
				exporter.IncreaseLogLevel("")
			case <-hup:
				logger.Info("file reloading.")
				if err := exporter.ReloadConfig(); err != nil {
					logger.Error(fmt.Sprintf("reload err: %s.", err))
				} else {
					logger.Info("file reloaded.")
				}
			case action := <-actionCh:
				switch action.actiontype {
				case ACTION_RELOAD:
					logger.Info("file reloading received.")
					if err := exporter.ReloadConfig(); err != nil {
						logger.Error(fmt.Sprintf("reload err: %s.", err))
						action.retCh <- err
					} else {
						logger.Info("file reloaded.")
						action.retCh <- nil
					}
				case ACTION_LOGLEVEL:
					if action.logLevel == "" {
						logger.Info("increase loglevel received.")
					} else {
						logger.Info("set loglevel received.")
					}
					exporter.IncreaseLogLevel(action.logLevel)
					action.retCh <- errors.New(exporter.GetLogLevel())
				}
			}
		}
	}()

	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	if exporter.Config().Globals.WebListenAddresses != "" {
		*toolkitFlags.WebListenAddresses = strings.Split(exporter.Config().Globals.WebListenAddresses, ",")
	}

	go func() {
		// Setup and start webserver.
		server := &http.Server{
			Handler: BuildHandler(exporter, actionCh),
		}
		if err := web.ListenAndServe(server, toolkitFlags, logger); err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}()

	for {
		select {
		case <-term:
			logger.Info("Received SIGTERM, exiting gracefully...")
			os.Exit(0)
		case <-srvc:
			os.Exit(1)
		}
	}
}
