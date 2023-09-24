package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "net/http/pprof"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
)

const (
	// Constant values
	metricsPublishingPort = ":9321"
	exporter_name         = "httpapi_exporter"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(metricsPublishingPort).String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose collector's internal metrics.").Default("/metrics").String()
	configFile    = kingpin.Flag("config.file", "Exporter configuration file.").Short('c').Default("config/config.yml").String()
	//debug_flag    = kingpin.Flag("debug", "debug connection checks.").Short('d').Default("false").Bool()
	dry_run = kingpin.Flag("dry-run", "Only check exporter configuration file and exit.").Short('n').Default("false").Bool()
	// alsologtostderr = kingpin.Flag("alsologtostderr", "log to standard error as well as files.").Default("true").Bool()
	target_name    = kingpin.Flag("target", "In dry-run mode specify the target name, else ignored.").Short('t').String()
	collector_name = kingpin.Flag("metric", "Specify the collector name restriction to collect, replace the collector_names set for each target.").Short('m').String()
)

func init() {
	prometheus.MustRegister(version.NewCollector(exporter_name))
}

func main() {

	logConfig := promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, &logConfig)
	kingpin.Version(version.Print(exporter_name)).VersionFlag.Short('V')
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promlog.New(&logConfig)
	level.Info(logger).Log("msg", fmt.Sprintf("Starting %s", exporter_name), "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())

	exporter, err := NewExporter(*configFile, logger, *collector_name)
	if err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error creating exporter: %s", err))
		os.Exit(1)
	}

	if *dry_run {
		level.Info(logger).Log("msg", "configuration OK.")
		// get the target if defined
		var t Target
		var err error
		if *target_name != "" {
			t, err = exporter.FindTarget(*target_name)
		} else {
			t, err = exporter.GetFirstTarget()
		}
		if err != nil {
			level.Error(logger).Log("errmsg", err)
			os.Exit(1)
		}
		level.Info(logger).Log("msg", fmt.Sprintf("try to collect target %s.", t.Name()))
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

		gatherer := prometheus.Gatherers{exporter.WithContext(ctx, t)}
		mfs, err := gatherer.Gather()
		if err != nil {
			level.Error(logger).Log("errmsg", fmt.Sprintf("Error gathering metrics: %v", err))
			if len(mfs) == 0 {
				os.Exit(1)
			}
		} else {
			level.Info(logger).Log("msg", "collect is OK. Dumping result to stdout.")
		}

		//dump metric to stdout
		enc := expfmt.NewEncoder(os.Stdout, expfmt.FmtText)

		for _, mf := range mfs {
			err := enc.Encode(mf)
			if err != nil {
				level.Error(logger).Log("Errmsg", err)
				break
			}
		}
		if closer, ok := enc.(expfmt.Closer); ok {
			// This in particular takes care of the final "# EOF\n" line for OpenMetrics.
			closer.Close()
		}
		level.Info(logger).Log("msg", "dry-run is over. Exiting.")
		os.Exit(0)
	}

	// Setup and start webserver.
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "OK", http.StatusOK) })
	http.HandleFunc("/", HomeHandlerFunc(*metricsPath, exporter))
	http.HandleFunc("/config", ConfigHandlerFunc(*metricsPath, exporter))
	http.HandleFunc("/status", StatusHandlerFunc(*metricsPath, exporter))
	http.HandleFunc("/targets", TargetsHandlerFunc(*metricsPath, exporter))
	http.Handle(*metricsPath, ExporterHandlerFor(exporter))
	// Expose exporter metrics separately, for debugging purposes.
	http.Handle("/httpapi_exporter_metrics", promhttp.Handler())

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "errmsg", err)
		os.Exit(1)
	}
}
