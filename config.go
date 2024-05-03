package main

import (
	"fmt"
	"reflect"
	"regexp"

	// "html/template"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	// "github.com/Masterminds/sprig/v3"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

// Load attempts to parse the given config file and return a Config object.
func LoadConfig(configFile string, logger log.Logger, collectorName string) (*Config, error) {
	level.Info(logger).Log("msg", fmt.Sprintf("Loading configuration from %s", configFile))
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	c := Config{
		configFile:    configFile,
		logger:        logger,
		collectorName: collectorName,
	}

	err = yaml.Unmarshal(buf, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

//
// Top-level config
//

// Config is a collection of targets and collectors.
type Config struct {
	Globals        *GlobalConfig          `yaml:"global"`
	CollectorFiles []string               `yaml:"collector_files,omitempty"`
	Targets        []*TargetConfig        `yaml:"targets,omitempty"`
	Collectors     []*CollectorConfig     `yaml:"collectors,omitempty"`
	HttpAPIConfig  map[string]*YAMLScript `yaml:"httpapi_config"`
	AuthConfigs    map[string]*AuthConfig `yaml:"auth_configs,omitempty"`

	configFile string
	logger     log.Logger
	// collectorName is a restriction: collectors set for a target are replaced by this only one.
	collectorName string

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target in `targets` must be defined")
	}

	// Load any externally defined collectors.
	if err := c.loadCollectorFiles(); err != nil {
		return err
	}

	if len(c.Collectors) == 0 {
		return fmt.Errorf("at least one collector in `collectors` must be defined")
	}

	// Populate collector references for the target/jobs.
	colls := make(map[string]*CollectorConfig)
	for id, coll := range c.Collectors {
		// Set the min interval to the global default if not explicitly set.
		if coll.MinInterval < 0 {
			coll.MinInterval = c.Globals.MinInterval
		}
		if _, found := colls[coll.Name]; found {
			return fmt.Errorf("duplicate collector name: %s", coll.Name)
		}
		colls[coll.Name] = coll
		// set metric prefix
		var prefix string
		for _, cs := range coll.CollectScripts {
			for _, a := range cs.Actions {
				reslist := a.GetMetrics()
				for _, res := range reslist {
					for _, metric := range res.mc {
						// for _, metric := range coll.Metrics {
						// add metric prefix  to all metrics name

						if res.maprefix != "" {
							prefix = res.maprefix
						} else if coll.MetricPrefix != "" {
							prefix = coll.MetricPrefix
						} else if c.Globals.MetricPrefix != "" {
							prefix = c.Globals.MetricPrefix
						}
						if !strings.HasPrefix(metric.Name, prefix) {
							metric.Name = fmt.Sprintf("%s_%s", prefix, metric.Name)
						}
					}
				}
			}
		}
		coll.id = fmt.Sprintf("%02d", id)
	}

	// read the target config with a TargetsFiles specfied
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			err := c.loadTargetsFiles(t.TargetsFiles)
			if err != nil {
				return err
			}
		} else {
			level.Info(c.logger).Log("msg", fmt.Sprintf("static target '%s' found", t.Name))
		}
	}
	targets := c.Targets
	c.Targets = nil
	// remove pseudo targets with a TargetsFiles
	for _, t := range targets {
		if len(t.TargetsFiles) == 0 {
			c.Targets = append(c.Targets, t)
		}
	}

	// check if a target nammed "default" exists
	// if not create one with default parameters from TargetConfig
	found := false
	for _, t := range c.Targets {
		if strings.ToLower(t.Name) == "default" {
			t.Name = "default"
			found = true
			break
		}
	}
	if !found {
		default_target := `
name: default
host: set_later
verifySSL: true
collectors:
  - ~.*_metrics
`
		t := &TargetConfig{}
		if err := yaml.Unmarshal([]byte(default_target), t); err != nil {
			return err
		}
		c.Targets = append(c.Targets, t)
		level.Info(c.logger).Log("msg", fmt.Sprintf("target '%s' added", t.Name))
	}

	for _, t := range c.Targets {
		// substitute the collector names list set in config by the value forced in command line argument
		if c.collectorName != "" {
			t.CollectorRefs = nil
			t.CollectorRefs = append(t.CollectorRefs, c.collectorName)
		}
		cs, err := resolveCollectorRefs(t.CollectorRefs, colls, fmt.Sprintf("target %q", t.Name))
		if err != nil {
			return err
		}
		t.collectors = cs

		// substitute AuthConfig name with auth config parameters
		if t.AuthName != "" {
			auth := c.FindAuthConfig(t.AuthName)
			if auth != nil {
				t.AuthConfig = *auth
			} else {
				return fmt.Errorf("auth_name '%s' not found for target '%s", t.AuthName, t.Name)
			}
		}
	}

	// Check for empty/duplicate target names
	tnames := make(map[string]interface{})
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			continue
		}
		if t.Name == "" {
			return fmt.Errorf("empty target name in static config %+v", t)
		}
		if _, ok := tnames[t.Name]; ok {
			return fmt.Errorf("duplicate target name %q in target %+v", t.Name, t)
		}
		tnames[t.Name] = nil

		// if t.ConnectionTimeout == 0 {
		// 	t.ConnectionTimeout = c.Globals.ConnectionTimeout
		// }

		if t.ScrapeTimeout == 0 {
			t.ScrapeTimeout = c.Globals.ScrapeTimeout
		}

		if t.QueryRetry == -1 {
			t.QueryRetry = c.Globals.QueryRetry
		}
	}

	// check HttpAPIConfig script:
	for name, sc := range c.HttpAPIConfig {
		if sc != nil {
			sc.name = name
			// have to set the action to play for play_script_action
			for _, a := range sc.Actions {
				if a.Type() == play_script_action || a.Type() == actions_action {
					if err := a.SetPlayAction(c.HttpAPIConfig); err != nil {
						return err
					}
				}
			}
		}
	}

	return checkOverflow(c.XXX, "config")
}

func (c *Config) FindAuthConfig(auth_name string) *AuthConfig {
	var auth *AuthConfig
	auth, found := c.AuthConfigs[auth_name]
	if !found {
		return nil
	}
	return auth
}

func GetScriptsDef(map_src map[string]*YAMLScript) map[string]ActionsList {
	var val ActionsList
	scdef := make(map[string]ActionsList, len(map_src)+1)
	for name, scr := range map_src {
		val = (ActionsList)(nil)
		if scr != nil {
			val = (ActionsList)(scr.Actions)
		}
		scdef[name] = val
	}
	return scdef
}

type dumpConfig struct {
	Globals        *GlobalConfig          `yaml:"global"`
	CollectorFiles []string               `yaml:"collector_files,omitempty"`
	Collectors     []*dumpCollectorConfig `yaml:"collectors,omitempty"`
	AuthConfigs    map[string]*AuthConfig `yaml:"auth_configs,omitempty"`
	// HttpAPIConfig  map[string]*ActionsList `yaml:"httpapi_config"`
	HttpAPIConfig map[string]ActionsList `yaml:"httpapi_config"`
}

// YAML marshals the config into YAML format.
func (c *Config) YAML() ([]byte, error) {
	dc := &dumpConfig{
		Globals:        c.Globals,
		AuthConfigs:    c.AuthConfigs,
		CollectorFiles: c.CollectorFiles,
		Collectors:     GetCollectorsDef(c.Collectors),
		HttpAPIConfig:  GetScriptsDef(c.HttpAPIConfig),
	}
	return yaml.Marshal(dc)
}

// loadCollectorFiles resolves all collector file globs to files and loads the collectors they define.
func (c *Config) loadCollectorFiles() error {
	baseDir := filepath.Dir(c.configFile)
	for _, cfglob := range c.CollectorFiles {
		// Resolve relative paths by joining them to the configuration file's directory.
		if len(cfglob) > 0 && !filepath.IsAbs(cfglob) {
			cfglob = filepath.Join(baseDir, cfglob)
		}

		// Resolve the glob to actual filenames.
		cfs, err := filepath.Glob(cfglob)
		level.Debug(c.logger).Log("msg", fmt.Sprintf("Checking collectors from %s", cfglob))
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error parsing collector files for %s: %s", cfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, cf := range cfs {
			level.Debug(c.logger).Log("msg", fmt.Sprintf("Loading collectors from %s", cf))
			buf, err := os.ReadFile(cf)
			if err != nil {
				return fmt.Errorf("reading collectors file %s: %s", cf, err)
			}

			cc := CollectorConfig{
				symtab: map[string]any{},
			}
			err = yaml.Unmarshal(buf, &cc)
			if err != nil {
				return fmt.Errorf("reading %s: %s", cf, err)
			}
			c.Collectors = append(c.Collectors, &cc)
			level.Info(c.logger).Log("msg", fmt.Sprintf("Loaded collector %s from %s", cc.Name, cf))
		}
	}

	return nil
}

// loadTargetsFiles resolves all targets file globs to files and loads the targets they define.
func (c *Config) loadTargetsFiles(targetFilepath []string) error {
	baseDir := filepath.Dir(c.configFile)
	for _, tfglob := range targetFilepath {
		// Resolve relative paths by joining them to the configuration file's directory.
		if len(tfglob) > 0 && !filepath.IsAbs(tfglob) {
			tfglob = filepath.Join(baseDir, tfglob)
		}

		// Resolve the glob to actual filenames.
		tfs, err := filepath.Glob(tfglob)
		level.Debug(c.logger).Log("msg", fmt.Sprintf("Checking targets from %s", tfglob))
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error resolving targets_files files for %s: %s", tfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, tf := range tfs {
			level.Debug(c.logger).Log("msg", fmt.Sprintf("Loading targets from %s", tf))
			buf, err := os.ReadFile(tf)
			if err != nil {
				return fmt.Errorf("reading targets_files for %s: %s", tf, err)
			}

			target := TargetConfig{}
			err = yaml.Unmarshal(buf, &target)
			if err != nil {
				return fmt.Errorf("parsing targets_files for %s: %s", tf, err)
			}
			target.setFromFile(tf)
			c.Targets = append(c.Targets, &target)
			level.Info(c.logger).Log("msg", fmt.Sprintf("Loaded target '%q' from %s", target.Name, tf))
		}
	}

	return nil
}

// GlobalConfig contains globally applicable defaults.
type GlobalConfig struct {
	MinInterval model.Duration `yaml:"min_interval"` // minimum interval between query executions, default is 0
	// ConnectionTimeout model.Duration `yaml:"connection_timeout"`    // connection timeout, target
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout"`        // per-scrape timeout, global
	TimeoutOffset   model.Duration `yaml:"scrape_timeout_offset"` // offset to subtract from timeout in seconds
	MetricPrefix    string         `yaml:"metric_prefix"`         // a prefix to ad dto all metric name; may be redefined in collector files
	QueryRetry      int            `yaml:"query_retry,omitempty"` // target specific number of times to retry a query
	InvalidHttpCode any            `yaml:"invalid_auth_code,omitempty"`
	ExporterName    string         `yaml:"exporter_name,omitempty"`

	invalid_auth_code []int
	// query_retry int
	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for GlobalConfig.
func (g *GlobalConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Default to running the queries on every scrape.
	g.MinInterval = model.Duration(0)
	// Default to 2 seconds, to connect to a target.
	// g.ConnectionTimeout = model.Duration(2 * time.Second)
	// Default to 10 seconds, since Prometheus has a 10 second scrape timeout default.
	g.ScrapeTimeout = model.Duration(10 * time.Second)
	// Default to .5 seconds.
	g.TimeoutOffset = model.Duration(500 * time.Millisecond)
	g.ExporterName = exporter_name

	// Default tp 3
	g.QueryRetry = 3

	// Default to httpapi
	g.MetricPrefix = "httpapi"

	type plain GlobalConfig
	if err := unmarshal((*plain)(g)); err != nil {
		return err
	}

	if g.TimeoutOffset <= 0 {
		return fmt.Errorf("global.scrape_timeout_offset must be strictly positive, have %s", g.TimeoutOffset)
	}
	// if g.ConnectionTimeout <= 0 {
	// 	return fmt.Errorf("global.connection_timeout must be strictly positive, have %s", g.ConnectionTimeout)
	// }
	if g.ScrapeTimeout <= 0 {
		return fmt.Errorf("global.connection_timeout must be strictly positive, have %s", g.ScrapeTimeout)
	}

	if g.InvalidHttpCode == nil {
		g.invalid_auth_code = []int{401, 403}
	} else {
		g.invalid_auth_code = buildStatus(g.InvalidHttpCode)
	}

	return checkOverflow(g.XXX, "global")
}

//
// Targets
//

// TargetConfig defines a url and a set of collectors to be executed on it.
type TargetConfig struct {
	Name       string     `yaml:"name"` // target name to connect to from prometheus
	Scheme     string     `yaml:"scheme"`
	Host       string     `yaml:"host"`
	Port       string     `yaml:"port,omitempty"`
	BaseUrl    string     `yaml:"baseUrl,omitempty"`
	AuthName   string     `yaml:"auth_name,omitempty"`
	AuthConfig AuthConfig `yaml:"auth_config,omitempty"`
	// Username          string             `yaml:"user,omitempty"`
	// Password          Secret             `yaml:"password,omitempty"` // data source definition to connect to
	// BasicAuth         ConvertibleBoolean `yaml:"basicAuth"`
	ProxyUrl        string `yaml:"proxy,omitempty"`
	VerifySSLString string `yaml:"verifySSL,omitempty"`
	// ConnectionTimeout model.Duration    `yaml:"connection_timeout,omitempty"` // connection timeout, per-target
	ScrapeTimeout model.Duration    `yaml:"scrape_timeout"`          // per-scrape timeout, global
	Labels        map[string]string `yaml:"labels,omitempty"`        // labels to apply to all metrics collected from the targets
	CollectorRefs []string          `yaml:"collectors"`              // names of collectors to execute on the target
	TargetsFiles  []string          `yaml:"targets_files,omitempty"` // slice of path and pattern for files that contains targets
	QueryRetry    int               `yaml:"query_retry,omitempty"`   // target specific number of times to retry a query

	collectors       []*CollectorConfig // resolved collector references
	fromFile         string             // filepath if loaded from targets_files pattern
	verifySSLUserSet bool
	verifySSL        ConvertibleBoolean
	// basicAuth  bool

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// Collectors returns the collectors referenced by the target, resolved.
func (t *TargetConfig) Collectors() []*CollectorConfig {
	return t.collectors
}

// set fromFile for target when read from targets_files directive
func (t *TargetConfig) setFromFile(file_path string) {
	t.fromFile = file_path
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for TargetConfig.
func (t *TargetConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain TargetConfig
	// set default value for target
	t.QueryRetry = -1
	// t.VerifySSL = ""
	// set default value for  VerifySSL
	t.verifySSL = true
	// by default value is not set by user; will be overwritten if user set a value
	t.verifySSLUserSet = false

	if err := unmarshal((*plain)(t)); err != nil {
		return err
	}

	// Check required fields
	if len(t.TargetsFiles) == 0 {
		if t.Name == "" {
			return fmt.Errorf("empty target name in target %+v", t)
		}

		if t.Scheme == "" {
			t.Scheme = "https"
		}
		if t.Port == "" {
			t.Port = "443"
		}
		if t.BaseUrl != "" {
			t.BaseUrl = strings.Trim(t.BaseUrl, "/")
		}

		if t.Host == "" {
			return fmt.Errorf("missing data_source_name for target %+v", t)
		}

		if t.VerifySSLString != "" {
			if err := t.verifySSL.UnmarshalJSON([]byte(t.VerifySSLString)); err != nil {
				return err
			}
			t.verifySSLUserSet = true
		}
		checkCollectorRefs(t.CollectorRefs, t.Name)

		if len(t.Labels) > 0 {
			err := t.checkLabelCollisions()
			if err != nil {
				return err
			}
		}
	} else {
		for _, file := range t.TargetsFiles {
			if file == "" {
				return fmt.Errorf("missing targets_files pattern")
			}
		}
	}
	if t.AuthConfig.Mode == "" {
		t.AuthConfig.Mode = "basic"
	}

	return checkOverflow(t.XXX, "target")
}

// checkLabelCollisions checks for label collisions between StaticConfig labels and Metric labels.
func (t *TargetConfig) checkLabelCollisions() error {
	sclabels := make(map[string]interface{})
	for _, l := range t.Labels {
		sclabels[l] = nil
	}

	for _, c := range t.collectors {
		// for _, m := range c.Metrics {
		for _, cs := range c.CollectScripts {
			for _, a := range cs.Actions {
				fmt.Printf("action type: %s", reflect.TypeOf(a))
				reslist := a.GetMetrics()
				for _, res := range reslist {
					for _, m := range res.mc {
						if keymap, ok := m.KeyLabels.(map[string]string); ok {
							for _, l := range keymap {
								if _, ok := sclabels[l]; ok {
									return fmt.Errorf(
										"label collision in target %q: label %q is defined both by a static_config and by metric %q of collector %q",
										t.Name, l, m.Name, c.Name)
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// method to build a temporary TargetConfig from "default" with host_name & and auth_name
func (t *TargetConfig) Clone(host_path string, auth_name string) (*TargetConfig, error) {
	new := &TargetConfig{
		Name:             host_path,
		Scheme:           t.Scheme,
		Host:             t.Host,
		Port:             t.Port,
		BaseUrl:          t.BaseUrl,
		AuthConfig:       t.AuthConfig,
		ProxyUrl:         t.ProxyUrl,
		ScrapeTimeout:    t.ScrapeTimeout,
		Labels:           t.Labels,
		QueryRetry:       t.QueryRetry,
		collectors:       t.collectors,
		verifySSLUserSet: t.verifySSLUserSet,
		verifySSL:        t.verifySSL,
	}

	url_elmt, err := url.Parse(host_path)
	if err != nil {
		return nil, err
	}
	if url_elmt.Scheme != "" && new.Scheme != url_elmt.Scheme {
		if url_elmt.Scheme == "https" || url_elmt.Scheme == "http" {
			new.Scheme = url_elmt.Scheme
		} else if url_elmt.Host == "" {
			// url.Parse for input "host.domain:port" builds .Scheme = "host.domain" .Opaque = "port"
			new.Host = url_elmt.Scheme
			if url_elmt.Opaque != "" {
				new.Port = url_elmt.Opaque
			}
		}
	}
	if url_elmt.Host == "" && url_elmt.Path != "" {
		new.Host = url_elmt.Path
	} else {
		if url_elmt.Host != "" {
			elmts := strings.Split(url_elmt.Host, ":")
			if new.Host != elmts[0] {
				new.Host = elmts[0]
			}
			if len(elmts) > 1 {
				new.Port = elmts[1]
			}
		}
	}
	if url_elmt.User.Username() != "" {
		new.AuthConfig.Username = url_elmt.User.Username()
		if tmp, set := url_elmt.User.Password(); set {
			new.AuthConfig.Password = Secret(tmp)
		}
		new.AuthConfig.Mode = "basic"
	}
	return new, nil
}

//
// Collectors
//

// type mapScript map[string]*YAMLScript

// func (m mapScript) MarshalText() (text []byte, err error) {
// 	var res []byte
// 	for name, sc := range m {
// 		b, err := yaml.Marshal(sc)
// 		if err != nil {
// 			return nil, err
// 		}
// 		res = append(res, []byte(name)...)
// 		res = append(res, []byte(":\n")...)
// 		b = bytes.Replace(b, []byte("|\n"), []byte(""), 1)
// 		res = append(res, b...)
// 	}
// 	return res, nil
// }

// CollectorConfig defines a set of metrics and how they are collected.
type CollectorConfig struct {
	Name         string         `yaml:"collector_name"`          // name of this collector
	MetricPrefix string         `yaml:"metric_prefix,omitempty"` // a prefix to ad dto all metric name; may be redefined in collector files
	MinInterval  model.Duration `yaml:"min_interval,omitempty"`  // minimum interval between query executions
	// Metrics      []*MetricConfig   `yaml:"metrics"`                // metrics/queries defined by this collector
	Templates      map[string]string      `yaml:"templates,omitempty"` // share custom templates/funcs for results templating
	CollectScripts map[string]*YAMLScript `yaml:"scripts,omitempty"`   // map of all independent scripts to collect metrics - each script can run in parallem
	// CollectScripts mapScript `yaml:"scripts,omitempty"` // map of all independent scripts to collect metrics - each script can run in parallem
	symtab map[string]any

	customTemplate *exporterTemplate // to store the custom Templates used by this collector
	// Metrics        []*MetricConfig    // metrics defined by this collector
	// id to print in log and to follow request action
	id string
	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for CollectorConfig.
func (c *CollectorConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Default to undefined (a negative value) so it can be overridden by the global default when not explicitly set.
	c.MinInterval = -1
	c.MetricPrefix = ""

	type plain CollectorConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	// if len(c.Metrics) == 0 {
	// 	return fmt.Errorf("no metrics defined for collector %q", c.Name)
	// }

	// build the default templates/funcs that my be used by all templates
	if len(c.Templates) > 0 {
		// c.customTemplate = template.New("default").Funcs(sprig.FuncMap())
		c.customTemplate = (*exporterTemplate)(template.New("default").Funcs(mymap()))
		if c.customTemplate == nil {
			return fmt.Errorf("for collector %s template is invalid", c.Name)
		}
		for name, tmpl := range c.Templates {
			def := "{{- define \"" + name + "\" }}" + strings.ReplaceAll(tmpl, "\n", "") + "{{ end -}}"
			ptr := (*template.Template)(c.customTemplate)
			if tmp_tpl, err := ptr.Parse(def); err != nil {
				return fmt.Errorf("for collector %s template %s is invalid: %s", c.Name, def, err)
			} else {
				c.customTemplate = (*exporterTemplate)(tmp_tpl)
			}
		}
	}

	if c.CollectScripts != nil {
		for collect_script_name, c_script := range c.CollectScripts {
			if c_script.name == "" {
				c_script.name = collect_script_name
			}
			if err := c_script.AddCustomTemplate(c.customTemplate); err != nil {
				err = fmt.Errorf("script %s: error with custom template: %s", c_script.name, err)
				return err
			}
		}
	}

	return checkOverflow(c.XXX, "collector")
}

type dumpCollectorConfig struct {
	Name           string                 `yaml:"collector_name"`          // name of this collector
	MetricPrefix   string                 `yaml:"metric_prefix,omitempty"` // a prefix to ad dto all metric name; may be redefined in collector files
	MinInterval    model.Duration         `yaml:"min_interval,omitempty"`  // minimum interval between query executions
	Templates      map[string]string      `yaml:"templates,omitempty"`     // share custom templates/funcs for results templating
	CollectScripts map[string]ActionsList `yaml:"scripts,omitempty"`       // map of all independent scripts to collect metrics - each script can run in parallem
}

func GetCollectorsDef(src_colls []*CollectorConfig) []*dumpCollectorConfig {
	colls := make([]*dumpCollectorConfig, len(src_colls))
	for idx, coll := range src_colls {
		colls[idx] = &dumpCollectorConfig{
			Name:           coll.Name,
			MetricPrefix:   coll.MetricPrefix,
			MinInterval:    coll.MinInterval,
			Templates:      coll.Templates,
			CollectScripts: GetScriptsDef(coll.CollectScripts),
		}
	}
	return colls
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for CollectorConfig.
// func (c *CollectorConfig) MarshalText() (text []byte, err error) {
// 	var res []byte
// 	// res = append(res, []byte(name)...)
// 	// res = append(res, []byte(":\n")...)
// 	// b = bytes.Replace(b, []byte("|\n"), []byte(""), 1)
// 	// res = append(res, b...)
// 	if b, err := yaml.Marshal(c.Name); err != nil {
// 		return nil, err
// 	} else {
// 		res = append(res, []byte("collector_name: ")...)
// 		res = append(res, b...)
// 	}

// 	if b, err := yaml.Marshal(c.MetricPrefix); err != nil {
// 		return nil, err
// 	} else {
// 		// res = append(res, []byte(name)...)
// 		res = append(res, []byte("metric_prefix: ")...)
// 		res = append(res, b...)
// 	}

// 	if b, err := yaml.Marshal(c.MinInterval); err != nil {
// 		return nil, err
// 	} else {
// 		// res = append(res, []byte(name)...)
// 		res = append(res, []byte("min_interval: ")...)
// 		res = append(res, b...)
// 	}

// 	if len(c.Templates) > 0 {
// 		if b, err := yaml.Marshal(c.Templates); err != nil {
// 			return nil, err
// 		} else {
// 			res = append(res, []byte("templates:\n")...)
// 			res = append(res, b...)
// 		}
// 	}
// 	if b, err := yaml.Marshal(c.CollectScripts); err != nil {
// 		return nil, err
// 	} else {
// 		res = append(res, []byte("scripts:\n")...)
// 		b = bytes.Replace(b, []byte("|\n"), []byte(""), 1)
// 		res = append(res, b...)
// 	}
// 	return res, nil
// }

// MetricConfig defines a Prometheus metric, the SQL query to populate it and the mapping of columns to metric
// keys/values.
type MetricConfig struct {
	Name       string `yaml:"metric_name"` // the Prometheus metric name
	TypeString string `yaml:"type"`        // the Prometheus metric type
	Help       string `yaml:"help"`        // the Prometheus metric help text
	// KeyLabels  map[string]string `yaml:"key_labels,omitempty"` // expose these atributes as labels from JSON object: format name: value with name and value that should be template
	KeyLabels any `yaml:"key_labels,omitempty"` // expose these atributes as labels from JSON object: format name: value with name and value that should be template
	// Labels       string            `yaml:"labels,omitempty"`        // expose these atributes as labels like key_labels but should be a variable template: format name: value with name and value that should be template
	StaticLabels map[string]string `yaml:"static_labels,omitempty"` // fixed key/value pairs as static labels
	ValueLabel   string            `yaml:"value_label,omitempty"`   // with multiple value columns, map their names under this label
	Values       map[string]string `yaml:"values"`                  // expose each of these columns as a value, keyed by column name
	// ResultFields []string          `yaml:"results,omitempty"`       // field name in JSON where to find a list of results
	Scope string `yaml:"scope,omitempty"` // var path where to collect data: shortcut for {{ .scope.path.var }}

	valueType      prometheus.ValueType // TypeString converted to prometheus.ValueType
	key_labels_map map[string]string
	key_labels     *Field
	// name *Field
	// help *Field
	// metric_type *Field
	// labels *Field
	// Catches all undefined fields and must be empty after parsing.
	// XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// ValueType returns the metric type, converted to a prometheus.ValueType.
func (m *MetricConfig) ValueType() prometheus.ValueType {
	return m.valueType
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for MetricConfig.
func (m *MetricConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain MetricConfig
	if err := unmarshal((*plain)(m)); err != nil {
		return err
	}

	// Check required fields
	if m.Name == "" {
		return fmt.Errorf("missing name for metric %+v", m)
	}
	// if val, err := NewField(m.Name, nil); err == nil {
	// 	m.name = val
	// } else {
	// 	return err
	// }

	if m.TypeString == "" {
		return fmt.Errorf("missing type for metric %q", m.Name)
	}
	// if val, err := NewField(m.TypeString, nil); err == nil {
	// 	m.metric_type = val
	// } else {
	// 	return err
	// }

	// help is not mandatory: so empty is valid
	// if m.Help == "" {
	// 	return fmt.Errorf("missing help for metric %q", m.Name)
	// }

	// if val, err := NewField(m.Help, nil); err == nil {
	// 	m.help = val
	// } else {
	// 	return err
	// }
	// if m.Labels != "" {
	// 	if val, err := NewField(m.Help, nil); err == nil {
	// 		m.help = val
	// 	} else {
	// 		return err
	// 	}
	// }

	switch strings.ToLower(m.TypeString) {
	case "counter":
		m.valueType = prometheus.CounterValue
	case "gauge":
		m.valueType = prometheus.GaugeValue
	default:
		return fmt.Errorf("unsupported metric type: %s", m.TypeString)
	}

	//	m.keyLabels := make(map[Label]*Label,0 len(m.KeyLabels))
	// Check for duplicate key labels
	if m.KeyLabels != nil {
		switch ktype := m.KeyLabels.(type) {
		case map[string]string:
			for key := range ktype {
				checkLabel(key, "metric", m.Name)
			}
			m.key_labels_map = ktype
		case map[string]any:
			m.key_labels_map = make(map[string]string, len(ktype))
			for key, val_raw := range ktype {
				checkLabel(key, "metric", m.Name)
				if val, ok := val_raw.(string); ok {
					m.key_labels_map[key] = val
				}
			}
		case string:
			if ktype != "" {
				if val, err := NewField(ktype, nil); err == nil {
					m.key_labels = val
				} else {
					return err
				}
			}
		default:
			return fmt.Errorf("key_labels should be a map[string][string] or Template(string) that will contain a map[string][string] for metric %q", m.Name)
		}
	}

	if len(m.Values) == 0 {
		return fmt.Errorf("no values defined for metric %q", m.Name)
	}

	if len(m.Values) > 1 {
		// Multiple value columns but no value label to identify them
		if m.ValueLabel == "" {
			return fmt.Errorf("value_label must be defined for metric with multiple values %q", m.Name)
		}
		checkLabel(m.ValueLabel, "value_label for metric", m.Name)
	}

	// return checkOverflow(m.XXX, "metric")
	return nil
}

// Secret special type for storing secrets.
type Secret string

// UnmarshalYAML implements the yaml.Unmarshaler interface for Secrets.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Secret
	return unmarshal((*plain)(s))
}

// MarshalYAML implements the yaml.Marshaler interface for Secrets.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s != "" {
		return "<secret>", nil
	}
	return nil, nil
}

// ConvertibleBoolean special type to retrive 1 yes true to boolean true
type ConvertibleBoolean bool

func (bit *ConvertibleBoolean) UnmarshalJSON(data []byte) error {
	asString := strings.ToLower(string(data))
	if asString == "1" || asString == "true" || asString == "yes" || asString == "on" {
		*bit = true
	} else if asString == "0" || asString == "false" || asString == "no" || asString == "off" {
		*bit = false
	} else {
		return fmt.Errorf("boolean unmarshal error: invalid input %s", asString)
	}
	return nil
}

type AuthConfig struct {
	Mode     string `yaml:"mode,omitempty"` // basic, encrypted, bearer
	Username string `yaml:"user,omitempty"`
	Password Secret `yaml:"password,omitempty"`
	Token    Secret `yaml:"token,omitempty"`
	authKey  string
}

func check_env_var(value string) string {
	if value != "" && strings.HasPrefix(value, "$env:") {
		value = os.Getenv(value[5:])
	}
	return value
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for authConfig
func (auth *AuthConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain AuthConfig
	if err := unmarshal((*plain)(auth)); err != nil {
		return err
	}

	// Check required fields
	if auth.Mode == "" {
		auth.Mode = "basic"
	} else {
		auth.Mode = strings.ToLower(auth.Mode)
		mode := make(map[string]int)
		for _, val := range []string{"basic", "token", "script"} {
			mode[val] = 1
		}
		if _, err := mode[auth.Mode]; !err {
			return fmt.Errorf("invalid mode auth %s", auth.Mode)
		}
	}
	if auth.Mode == "token" && auth.Token == "" {
		return fmt.Errorf("token not set with auth mode 'token'")
	}

	// auth.Username == $env:VAR_NAME
	auth.Username = check_env_var(auth.Username)
	auth.Password = Secret(check_env_var(string(auth.Password)))
	auth.Token = Secret(check_env_var(string(auth.Token)))

	return nil
}

// *************************************************************************************************
func checkCollectorRefs(collectorRefs []string, ctx string) error {
	// At least one collector, no duplicates
	if len(collectorRefs) == 0 {
		return fmt.Errorf("no collectors defined for %s", ctx)
	}
	for i, ci := range collectorRefs {
		for _, cj := range collectorRefs[i+1:] {
			if ci == cj {
				return fmt.Errorf("duplicate collector reference %q in %s", ci, ctx)
			}
		}
	}
	return nil
}

func resolveCollectorRefs(
	collectorRefs []string, collectors map[string]*CollectorConfig, ctx string) ([]*CollectorConfig, error) {
	resolved := make([]*CollectorConfig, 0, len(collectorRefs))
	for _, cref := range collectorRefs {
		// check if cref(a collector name) is a pattern or not
		if strings.HasPrefix(cref, "~") {
			pat := regexp.MustCompile(cref[1:])
			for c_name, c := range collectors {
				if pat.MatchString(c_name) {
					resolved = append(resolved, c)
				}
			}
		} else if strings.HasPrefix(cref, "!~") {
			pat := regexp.MustCompile(cref[2:])
			for c_name, c := range collectors {
				if !pat.MatchString(c_name) {
					resolved = append(resolved, c)
				}
			}
		} else {
			c, found := collectors[cref]
			if !found {
				return nil, fmt.Errorf("unknown collector %q referenced in %s", cref, ctx)
			}
			resolved = append(resolved, c)
		}
	}
	return resolved, nil
}

func checkLabel(label string, ctx ...string) error {
	if label == "" {
		return fmt.Errorf("empty label defined in %s", strings.Join(ctx, " "))
	}
	if label == "job" || label == "instance" {
		return fmt.Errorf("reserved label %q redefined in %s", label, strings.Join(ctx, " "))
	}
	return nil
}

func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields '%s' in '%s'", strings.Join(keys, ", "), ctx)
	}
	return nil
}
