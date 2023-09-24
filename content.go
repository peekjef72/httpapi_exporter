package main

import (
	"fmt"
	"html/template"
	"net/http"
	"runtime"

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
          <div><a href="/healthz">Health</a></div>
          <div><a href="{{ .MetricsPath }}">Metrics</a></div>
          <div><a href="/config">Configuration</a></div>
          <div><a href="/targets">Targets</a></div>
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
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion string
}
type tdata struct {
	ExporterName string
	MetricsPath  string
	DocsUrl      string

	// `/config` only
	Config string

	// `:targets` only
	Targets string

	// status
	Version versionInfo
	// `/error` only
	Err error
}

var (
	allTemplates    = template.Must(template.New("").Parse(templates))
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

// ConfigHandlerFunc is the HTTP handler for the `/config` page. It outputs the configuration marshaled in YAML format.
func ConfigHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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

// ConfigHandlerFunc is the HTTP handler for the `/config` page. It outputs the configuration marshaled in YAML format.
func StatusHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		vinfos := versionInfo{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: runtime.Version(),
		}

		statusTemplate.Execute(w, &tdata{
			ExporterName: exporter.Config().Globals.ExporterName,
			MetricsPath:  metricsPath,
			DocsUrl:      docsUrl,
			Version:      vinfos,
		})
	}
}

// TargetsHandlerFunc is the HTTP handler for the `/target` page. It outputs the targets configuration marshaled in YAML format.
func TargetsHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var targets_cfg []byte
		var err error
		c := exporter.Config()
		// for _, t := range c.Targets {
		targets_cfg, err = yaml.Marshal(c.Targets)
		if err != nil {
			// content = nil
			HandleError(0, err, metricsPath, exporter, w, r)
			return
		}
		// targets_cfg = append(targets_cfg, content...)
		// }
		targetsTemplate.Execute(w, &tdata{
			ExporterName: exporter.Config().Globals.ExporterName,
			MetricsPath:  metricsPath,
			DocsUrl:      docsUrl,
			Targets:      string(targets_cfg),
		})
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
