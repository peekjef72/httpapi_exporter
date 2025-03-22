package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"strings"

	"github.com/prometheus/common/version"
	"gopkg.in/yaml.v3"
)

const (
	docsUrl   = "https://github.com/peekjef72/httpapi_exporter#readme"
	templates = `
    {{ define "page" -}}
      <html>
      <head>
        <title>Prometheus {{ .ExporterName }}</title>
        <style type="text/css">
          body { margin: 0; font-family: "Helvetica Neue", Helvetica, Arial, sans-serif; font-size: 14px; line-height: 1.42857143; color: #333; background-color: #fff; }
          .navbar { display: flex; background-color: #222; margin: 0; border-width: 0 0 1px; border-style: solid; border-color: #080808; }
          .navbar > * { margin: 0; padding: 15px; }
          .navbar * { line-height: 20px; color: #9d9d9d; }
          .navbar a { text-decoration: none; }
          .navbar a:hover, .navbar a:focus { color: #fff; }
          .navbar-header { font-size: 18px; }
          body > * { margin: 15px; padding: 0; }
          pre { padding: 10px; font-size: 13px; background-color: #f5f5f5; border: 1px solid #ccc; }
          h1, h2 { font-weight: 500; }
          a { color: #337ab7; }
          a:hover, a:focus { color: #23527c; }
		  table { border: 1px solid #edd2e6; border-collapse: collapse; margin-bottom: 1rem; width: 80%; }
		  tr { border: 1px solid #edd2e6; padding: 0.3rem; text-align: left; width: 35%; }
		  th { border: 1px solid #edd2e6; padding: 0.3rem; }
		  td { border: 1px solid #edd2e6; padding: 0.3rem; }
		  .odd { background-color: rgba(0,0,0,.05); }
        </style>
      </head>
      <body>
        <div class="navbar">
          <div class="navbar-header"><a href="/">Prometheus {{ .ExporterName }}</a></div>
          <div><a href="/health">Health</a></div>
          <div><a href="{{ .MetricsPath }}">Metrics</a></div>
          <div><a href="/config">Configuration</a></div>
          <div><a href="/targets">Targets</a></div>
          <div><a href="/loglevel">loglevel</a></div>
          <div><a href="/status">Status</a></div>
          <div><a href="/debug/pprof">Profiling</a></div>
          <div><a href="/httpapi_exporter_metrics">Exporter Metrics</a></div>
          <div><a href="{{ .DocsUrl }}">Help</a></div>
        </div>
        {{template "content" .}}
      </body>
      </html>
    {{- end }}

    {{ define "content.home" -}}
      <p>This is a <a href="{{ .DocsUrl }}">Prometheus {{ .ExporterName }}</a> instance.
        You are probably looking for its <a href="{{ .MetricsPath }}">metrics</a> handler.</p>
    {{- end }}

	{{ define "content.health" -}}
      <H2>{{ .Message }}</H2>
    {{- end }}

    {{ define "content.config" -}}
      <h2>Configuration</h2>
      <pre>{{ .Config }}</pre>
    {{- end }}

    {{ define "content.targets" -}}
      <h2>Targets</h2>
      <pre>{{ .Targets }}</pre>
    {{- end }}

	{{ define "content.status" -}}
	<h2>Build Information</h2>
	<table>
	  	<tbody>
			<tr class="odd" >
				<th>Version</th>
				<td>{{ .Version.Version }}</td>
			</tr>
			<tr>
				<th>Revision</th>
				<td>{{ .Version.Revision }}</td>
			</tr>
			<tr class="odd" >
				<th>Branch</th>
				<td>{{ .Version.Branch }}</td>
			</tr>
			<tr>
				<th>BuildUser</th>
				<td>{{ .Version.BuildUser }}</td>
			</tr>
			<tr class="odd" >
				<th>BuildDate</th>
				<td>{{ .Version.BuildDate }}</td>
			</tr>
			<tr>
				<th>GoVersion</th>
				<td>{{ .Version.GoVersion }}</td>
			</tr>
			<tr class="odd" >
				<th>Server start</th>
				<td>{{ .Version.StartTime }}</td>
			</tr>
			<tr>
				<th>Server last reload</th>
				<td>{{ .Version.ReloadTime }}</td>
			</tr>
		</tbody>
	</table>
  {{- end }}

    {{ define "content.error" -}}
      <h2>Error</h2>
      <pre>{{ .Err }}</pre>
    {{- end }}
    `
)

type versionInfo struct {
	Version    string `json:"version"`
	Revision   string `json:"revision"`
	Branch     string `json:"branch"`
	BuildUser  string `json:"build_user"`
	BuildDate  string `json:"build_date"`
	GoVersion  string `json:"go_version"`
	StartTime  string `json:"start_time"`
	ReloadTime string `json:"reload_time"`
}
type tdata struct {
	ExporterName string
	MetricsPath  string
	DocsUrl      string

	// `/config` only
	Config string

	// `/targets` only
	Targets string

	// status
	Version versionInfo
	// health - loglevel
	Message string
	// `/error` only
	Err error
}

var (
	allTemplates    = template.Must(template.New("").Parse(templates))
	healthTemplate  = pageTemplate("health")
	homeTemplate    = pageTemplate("home")
	configTemplate  = pageTemplate("config")
	targetsTemplate = pageTemplate("targets")
	statusTemplate  = pageTemplate("status")
	errorTemplate   = pageTemplate("error")
)

func pageTemplate(name string) *template.Template {
	pageTemplate := fmt.Sprintf(`{{define "content"}}{{template "content.%s" .}}{{end}}{{template "page" .}}`, name)
	return template.Must(template.Must(allTemplates.Clone()).Parse(pageTemplate))
}

// HomeHandlerFunc is the HTTP handler for the home page (`/`).
func HomeHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		homeTemplate.Execute(w, &tdata{
			ExporterName: exporter.Config().Globals.ExporterName,
			MetricsPath:  metricsPath,
			DocsUrl:      docsUrl,
		})
	}
}

func HealthHandlerfunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var status []byte
		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {
			w.Header().Set(contentTypeHeader, applicationJSON)
			status = []byte(`{"message":"ok","status": 1,"data": {"status":"ok"}}`)
			w.Header().Set(contentLengthHeader, fmt.Sprint(len(status)))
		} else if strings.Contains(accept_type, textPLAIN) {
			w.Header().Set(contentTypeHeader, textPLAIN)
			status = []byte("OK")
		} else {
			w.Header().Set(contentTypeHeader, textHTML)
			healthTemplate.Execute(w, &tdata{
				ExporterName: exporter.Config().Globals.ExporterName,
				MetricsPath:  metricsPath,
				DocsUrl:      docsUrl,
				Message:      "OK",
			})
			return
		}
		w.Write(status)
	}
}

// ConfigHandlerFunc is the HTTP handler for the `/config` page. It outputs the configuration marshaled in YAML format.
func ConfigHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {
			conf, err := exporter.Config().JSON()
			if err != nil {
				HandleError(0, err, metricsPath, exporter, w, r)
				return
			}
			w.Header().Set(contentTypeHeader, applicationJSON)
			w.Header().Set(contentLengthHeader, fmt.Sprint(len(conf)))
			w.Write(conf)
		} else {
			config, err := exporter.Config().YAML()
			if err != nil {
				HandleError(0, err, metricsPath, exporter, w, r)
				return
			}
			configTemplate.Execute(w, &tdata{
				ExporterName: exporter.Config().Globals.ExporterName,
				MetricsPath:  metricsPath,
				DocsUrl:      docsUrl,
				Config:       string(config),
			})
		}
	}
}

// ReloadHandlerFunc is the HTTP handler for the POST reload entry point (`/reload`).
func ReloadHandlerFunc(metricsPath string, exporter Exporter, reloadCh chan<- actionMsg) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var result []byte

		if r.Method != "POST" {
			exporter.Logger().Info(
				"received invalid method on /reload", "client", r.RemoteAddr)
			HandleError(http.StatusMethodNotAllowed, errors.New("this endpoint requires a POST request"), metricsPath, exporter, w, r)
			return
		}

		exporter.Logger().Info(
			"received /reload from %s", "client", r.RemoteAddr)

		msg := actionMsg{
			actiontype: ACTION_RELOAD,
			retCh:      make(chan error),
		}
		reloadCh <- msg
		if err := <-msg.retCh; err != nil {
			HandleError(http.StatusInternalServerError, err, metricsPath, exporter, w, r)
		}

		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {
			accept_type = applicationJSON
		} else if strings.Contains(accept_type, textPLAIN) {
			accept_type = textPLAIN
		} else {
			accept_type = textHTML
		}

		switch accept_type {
		case textPLAIN:
			w.Header().Set(contentTypeHeader, textPLAIN)
			result = []byte(`OK reload asked.`)
		case applicationJSON:
			w.Header().Set(contentTypeHeader, applicationJSON)
			result = []byte(`{"message":"ok","status": 1,"data": {"reload": true}}`)
			w.Header().Set(contentLengthHeader, fmt.Sprint(len(result)))
		default:
			w.Header().Set(contentTypeHeader, textHTML)
			healthTemplate.Execute(w, &tdata{
				ExporterName: exporter.Config().Globals.ExporterName,
				MetricsPath:  metricsPath,
				DocsUrl:      docsUrl,
				Message:      `OK reload asked.`,
			})
			return
		}
		w.Write(result)
	}
}

// LogLevelHandlerFunc is the HTTP handler for the POST loglevel entry point (`/loglevel`).
func LogLevelHandlerFunc(metricsPath string, exporter Exporter, reloadCh chan<- actionMsg, path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var result []byte

		exporter.Logger().Info(
			"received /loglevel", "client", r.RemoteAddr)

		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {
			accept_type = applicationJSON
		} else if strings.Contains(accept_type, textPLAIN) {
			accept_type = textPLAIN
		} else {
			accept_type = textHTML
		}

		switch r.Method {
		case "GET":
			switch accept_type {
			case textPLAIN:
				w.Header().Set(contentTypeHeader, textPLAIN)
				result = []byte(
					fmt.Sprintf("loglevel is currently set to %s", exporter.GetLogLevel()))
			case applicationJSON:
				w.Header().Set(contentTypeHeader, applicationJSON)
				result = []byte(
					fmt.Sprintf(`{"message":"ok","status": 1,"data": {"loglevel": "%s"}}`, exporter.GetLogLevel()))
				w.Header().Set(contentLengthHeader, fmt.Sprint(len(result)))
			default:
				w.Header().Set(contentTypeHeader, textHTML)
				healthTemplate.Execute(w, &tdata{
					ExporterName: exporter.Config().Globals.ExporterName,
					MetricsPath:  metricsPath,
					DocsUrl:      docsUrl,
					Message:      fmt.Sprintf("loglevel is currently set to %s", exporter.GetLogLevel()),
				})
				return
			}
			w.Write(result)
		case "POST":
			ctxval, ok := r.Context().Value(ctxKey{}).(*ctxValue)
			if !ok {
				err := errors.New("invalid context received")
				HandleError(http.StatusInternalServerError, err, metricsPath, exporter, w, r)
				return

			}
			msg := actionMsg{
				actiontype: ACTION_LOGLEVEL,
				logLevel:   strings.ToLower(ctxval.path),
				retCh:      make(chan error),
			}
			reloadCh <- msg
			if err := <-msg.retCh; err != nil {
				switch accept_type {
				case textPLAIN:
					w.Header().Set(contentTypeHeader, textPLAIN)
					result = []byte(
						fmt.Sprintf("OK loglevel set to %s", exporter.GetLogLevel()))
				case applicationJSON:
					w.Header().Set(contentTypeHeader, applicationJSON)
					result = []byte(
						fmt.Sprintf(`{"message":"ok","status": 1,"data": {"loglevel": "%s"}}`, exporter.GetLogLevel()))
					w.Header().Set(contentLengthHeader, fmt.Sprint(len(result)))
				default:
					w.Header().Set(contentTypeHeader, textHTML)
					healthTemplate.Execute(w, &tdata{
						ExporterName: exporter.Config().Globals.ExporterName,
						MetricsPath:  metricsPath,
						DocsUrl:      docsUrl,
						Message:      fmt.Sprintf("OK loglevel set to %s", exporter.GetLogLevel()),
					})
					return
				}
				w.Write(result)
			} else {
				HandleError(http.StatusInternalServerError, errors.New("KO something wrong with increase loglevel"), metricsPath, exporter, w, r)
			}
		default:
			exporter.Logger().Info(
				"received invalid method on /loglevel", "client", r.RemoteAddr)
			HandleError(http.StatusMethodNotAllowed, errors.New("this endpoint requires a GET or POST request"), metricsPath, exporter, w, r)
			return
		}
	}
}

// StatusHandlerFunc is the HTTP handler for the `/status` page. It outputs the status of exporter.
func StatusHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		vinfos := versionInfo{
			Version:    version.Version,
			Revision:   version.Revision,
			Branch:     version.Branch,
			BuildUser:  version.BuildUser,
			BuildDate:  version.BuildDate,
			GoVersion:  runtime.Version(),
			StartTime:  exporter.GetStartTime(),
			ReloadTime: exporter.GetReloadTime(),
		}

		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {
			res, err := json.Marshal(vinfos)
			if err != nil {
				HandleError(http.StatusBadRequest, err, metricsPath, exporter, w, r)
				return
			}
			w.Header().Set(contentTypeHeader, applicationJSON)
			w.Header().Set(contentLengthHeader, fmt.Sprint(len(res)))
			w.WriteHeader(http.StatusOK)
			w.Write(res)
		} else {
			w.Header().Set(contentTypeHeader, textHTML)
			statusTemplate.Execute(w, &tdata{
				ExporterName: exporter.Config().Globals.ExporterName,
				MetricsPath:  metricsPath,
				DocsUrl:      docsUrl,
				Version:      vinfos,
			})
		}
	}
}

// TargetsHandlerFunc is the HTTP handler for the `/target` page. It outputs the targets configuration marshaled in YAML format.
func TargetsHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		type targets struct {
			Tgs []*TargetConfig `yaml:"targets" json:"targets"`
		}
		var (
			tgs         *targets
			targets_cfg []byte
			err         error
		)

		ctxval, ok := r.Context().Value(ctxKey{}).(*ctxValue)
		if !ok {
			err := errors.New("invalid context received")
			HandleError(http.StatusInternalServerError, err, metricsPath, exporter, w, r)
			return

		}

		c := exporter.Config()
		if ctxval.path != "" {
			if tg, err := exporter.FindTarget(ctxval.path); err == nil {
				tgl := make([]*TargetConfig, 1)
				tgl[0] = tg.Config()

				tgs = &targets{
					Tgs: tgl,
				}
			} else {
				HandleError(http.StatusNotFound, errors.New(`target not found`), metricsPath, exporter, w, r)
				return
			}
		} else {
			tgs = &targets{
				Tgs: c.Targets,
			}
		}
		accept_type := r.Header.Get(acceptHeader)
		if strings.Contains(accept_type, applicationJSON) {

			targets_cfg, err = json.Marshal(tgs)
			if err != nil {
				HandleError(0, err, metricsPath, exporter, w, r)
				return
			}
			w.Header().Set(contentTypeHeader, applicationJSON)
			w.Header().Set(contentLengthHeader, fmt.Sprint(len(targets_cfg)))
			w.WriteHeader(http.StatusOK)
			w.Write(targets_cfg)
		} else {
			targets_cfg, err = yaml.Marshal(c.Targets)
			if err != nil {
				HandleError(0, err, metricsPath, exporter, w, r)
				return
			}
			w.Header().Set(contentTypeHeader, textHTML)
			targetsTemplate.Execute(w, &tdata{
				ExporterName: exporter.Config().Globals.ExporterName,
				MetricsPath:  metricsPath,
				DocsUrl:      docsUrl,
				Targets:      string(targets_cfg),
			})
		}
	}
}

// HandleError is an error handler that other handlers defer to in case of error. It is important to not have written
// anything to w before calling HandleError(), or the 500 status code won't be set (and the content might be mixed up).
func HandleError(status int, err error, metricsPath string, exporter Exporter, w http.ResponseWriter, r *http.Request) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	w.WriteHeader(status)
	errorTemplate.Execute(w, &tdata{
		ExporterName: exporter.Config().Globals.ExporterName,
		MetricsPath:  metricsPath,
		DocsUrl:      docsUrl,
		Err:          err,
	})
}
