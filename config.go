package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"

	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	mytemplate "github.com/peekjef72/httpapi_exporter/template"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

// Load attempts to parse the given config file and return a Config object.
func LoadConfig(configFile string, logger *slog.Logger, collectorName string) (*Config, error) {
	logger.Info(fmt.Sprintf("Loading configuration from %s", configFile))
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

// type httpAPIConfig map[string]*YAMLScript
// type Profile map[string]*YAMLScript
type ScriptConfig map[string]*YAMLScript

type Profile struct {
	MetricPrefix string                 `yaml:"metric_prefix,omitempty" json:"metric_prefix,omitempty"`
	Scripts      map[string]*YAMLScript `yaml:"scripts" json:"scripts"`
}

// Top-level config

// Config is a collection of targets and collectors.
type Config struct {
	Globals        *GlobalConfig          `yaml:"global"`
	CollectorFiles []string               `yaml:"collector_files,omitempty"`
	Targets        []*TargetConfig        `yaml:"targets,omitempty"`
	Collectors     []*CollectorConfig     `yaml:"collectors,omitempty"`
	Profiles       map[string]*Profile    `yaml:"profiles"`
	ProfileFiles   []string               `yaml:"profiles_file_config"`
	AuthConfigs    map[string]*AuthConfig `yaml:"auth_configs,omitempty"`
	// obsolete will be remove in next version
	HttpAPIConfigOld map[string]*YAMLScript `yaml:"httpapi_config"`

	configFile string
	logger     *slog.Logger
	// collectorName is a restriction: collectors set for a target are replaced by this only one.
	collectorName string
	collectors    map[string]*CollectorConfig

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
						if name, err := NewField(metric.Name, nil); err == nil {
							metric.name = name
						} else {
							return err
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
			c.logger.Info(fmt.Sprintf("static target '%s' found", t.Name))
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
host: template
verifySSL: true
profile: default
collectors:
  - ~.*
`
		t := &TargetConfig{}
		if err := yaml.Unmarshal([]byte(default_target), t); err != nil {
			return err
		}
		c.Targets = append(c.Targets, t)
		c.logger.Info(fmt.Sprintf("target '%s' added", t.Name))
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
		if len(cs) == 0 {
			return fmt.Errorf("target %s has no collector defined", t.Name)
		} else {
			c.logger.Debug(fmt.Sprintf("target '%s' has collectors", t.Name), "collectors", (CollectorConfigList)(cs).String())
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

	// convert old format HttpAPIConfig to new one
	if c.HttpAPIConfigOld != nil {
		if c.Profiles == nil {
			c.Profiles = make(map[string]*Profile)
		}
		if _, found := c.Profiles["default"]; found {
			return fmt.Errorf("incompatible: old config httpapi_config and profiles[\"default\"] found")
		}
		profile := &Profile{
			MetricPrefix: c.Globals.MetricPrefix,
			Scripts:      (ScriptConfig)(c.HttpAPIConfigOld),
		}
		c.Profiles["default"] = profile
	}

	// Load any externally defined collectors.
	if err := c.loadProfileFiles(); err != nil {
		return err
	}

	// check each profile scripts:
	for profile_name, profile := range c.Profiles {
		if profile.MetricPrefix == "" {
			profile.MetricPrefix = c.Globals.MetricPrefix
		}
		if len(profile.Scripts) == 0 {
			return fmt.Errorf("profile '%s' has an empty scripts section", profile_name)
		}
		for name, sc := range profile.Scripts {
			if sc != nil {
				sc.name = name
				// have to set the action to play for play_script_action
				for _, a := range sc.Actions {
					if a.Type() == play_script_action || a.Type() == actions_action {
						if err := a.SetPlayAction(profile.Scripts); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	// Check for empty/duplicate target names
	// check for valid "known" profile name
	tnames := make(map[string]interface{})
	var (
		default_profile      *Profile
		default_profile_name string
	)
	if len(c.Profiles) == 1 {
		for profile_name, profile := range c.Profiles {
			default_profile = profile
			default_profile_name = profile_name
			break
		}
	}

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

		if t.ScrapeTimeout == 0 {
			t.ScrapeTimeout = c.Globals.ScrapeTimeout
		}

		if t.QueryRetry == -1 {
			t.QueryRetry = c.Globals.QueryRetry
		}
		if profile, found := c.Profiles[t.ProfileName]; !found {
			if default_profile != nil {
				var msg string
				if t.ProfileName != "" {
					msg = fmt.Sprintf("target '%s' profile set '%s' not found. reset to default.", t.Name, t.ProfileName)
				} else {
					msg = fmt.Sprintf("target '%s' not profile set. reset to default.", t.Name)
				}
				c.logger.Warn(msg, "profile", t.ProfileName)
				t.profile = default_profile
				t.ProfileName = default_profile_name
			} else {
				return fmt.Errorf("profile %q not found for target name %q", t.ProfileName, t.Name)
			}
		} else {
			t.profile = profile
		}
	}
	// reserve collector ref;
	c.collectors = colls

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

func (c *Config) FindCollector(collector_name string) *CollectorConfig {
	var coll *CollectorConfig
	coll, found := c.collectors[collector_name]
	if !found {
		return nil
	}
	return coll
}

type DumpProfile struct {
	MetricPrefix string                 `yaml:"metric_prefix,omitempty" json:"metric_prefix,omitempty"`
	Scripts      map[string]ActionsList `yaml:"scripts" json:"scripts"`
}
type DumpScriptConfig map[string]ActionsList

func GetProfilesDef(profiles map[string]*Profile) map[string]*DumpProfile {
	profile_def := make(map[string]*DumpProfile, len(profiles)+1)

	for profile_name, profile := range profiles {
		new_prof := &DumpProfile{
			MetricPrefix: profile.MetricPrefix,
			Scripts:      GetScriptsDef(profile.Scripts),
		}
		profile_def[profile_name] = new_prof
	}
	return profile_def
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
	Globals        *GlobalConfig           `yaml:"global" json:"global"`
	CollectorFiles []string                `yaml:"collector_files,omitempty" json:"collector_files,omitempty"`
	Collectors     []*dumpCollectorConfig  `yaml:"collectors,omitempty" json:"collectors,omitempty"`
	AuthConfigs    map[string]*AuthConfig  `yaml:"auth_configs,omitempty" json:"auth_configs,omitempty"`
	Profiles       map[string]*DumpProfile `yaml:"profiles" json:"profiles"`
}

// YAML marshals the config into YAML format.
func (c *Config) YAML() ([]byte, error) {
	dc := &dumpConfig{
		Globals:        c.Globals,
		AuthConfigs:    c.AuthConfigs,
		CollectorFiles: c.CollectorFiles,
		Collectors:     GetCollectorsDef(c.Collectors),
		Profiles:       GetProfilesDef(c.Profiles),
		//		HttpAPIConfig:  GetScriptsDef(c.HttpAPIConfig),
	}
	return yaml.Marshal(dc)
}

// JSON marshals the config into JSON format.
func (c *Config) JSON() ([]byte, error) {
	type fullConf struct {
		Config *dumpConfig `json:"config"`
	}
	fc := &fullConf{
		Config: &dumpConfig{
			Globals:        c.Globals,
			AuthConfigs:    c.AuthConfigs,
			CollectorFiles: c.CollectorFiles,
			Collectors:     GetCollectorsDef(c.Collectors),
			Profiles:       GetProfilesDef(c.Profiles),
			//			HttpAPIConfig:  GetScriptsDef(c.HttpAPIConfig),
		},
	}
	return json.Marshal(fc)
}

// loadProfileFiles resolves all profile file globs to files and loads the profiles they define.
func (c *Config) loadProfileFiles() error {
	baseDir := filepath.Dir(c.configFile)
	for _, pfglob := range c.ProfileFiles {
		// Resolve relative paths by joining them to the configuration file's directory.
		if len(pfglob) > 0 && !filepath.IsAbs(pfglob) {
			pfglob = filepath.Join(baseDir, pfglob)
		}

		// Resolve the glob to actual filenames.
		pfs, err := filepath.Glob(pfglob)
		c.logger.Debug(fmt.Sprintf("Checking profiles from %s", pfglob))
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error parsing profile files for %s: %s", pfglob, err)
		}

		type Profiles map[string]*Profile
		// And load the Profiles defined in each file.
		for _, pf := range pfs {
			c.logger.Debug(fmt.Sprintf("Loading profiles from %s", pf))
			buf, err := os.ReadFile(pf)
			if err != nil {
				return fmt.Errorf("reading profiles file %s: %s", pf, err)
			}

			var profiles Profiles
			err = yaml.Unmarshal(buf, &profiles)
			if err != nil {
				return fmt.Errorf("reading %s: %s", pf, err)
			}
			if c.Profiles == nil {
				c.Profiles = make(map[string]*Profile)
			}
			for p_name, p := range profiles {
				c.logger.Info(fmt.Sprintf("Loaded profile %s from %s", p_name, pf))
				if _, found := c.Profiles[p_name]; found {
					return fmt.Errorf("profile %s already defined", p_name)
				}
				c.Profiles[p_name] = p
			}
		}
	}

	return nil
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
		c.logger.Debug(fmt.Sprintf("Checking collectors from %s", cfglob))
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error parsing collector files for %s: %s", cfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, cf := range cfs {
			c.logger.Debug(fmt.Sprintf("Loading collectors from %s", cf))
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
			c.logger.Info(fmt.Sprintf("Loaded collector %s from %s", cc.Name, cf))
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
		c.logger.Debug(fmt.Sprintf("Checking targets from %s", tfglob))
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error resolving targets_files files for %s: %s", tfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, tf := range tfs {
			c.logger.Debug(fmt.Sprintf("Loading targets from %s", tf))
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
			c.logger.Info(fmt.Sprintf("Loaded target '%q' from %s", target.Name, tf))
		}
	}

	return nil
}

const (
	tls_version_upto_1_2 uint = 0x1 // tls_upto_1.2
	tls_version_1_2      uint = 0x2 // tls_1.2
	tls_version_1_3      uint = 0x4 // tls_1.3
)

// GlobalConfig contains globally applicable defaults.
type GlobalConfig struct {
	MinInterval     model.Duration `yaml:"min_interval" json:"min_interval"`                   // minimum interval between query executions, default is 0
	ScrapeTimeout   model.Duration `yaml:"scrape_timeout" json:"scrape_timeout"`               // per-scrape timeout, global
	TimeoutOffset   model.Duration `yaml:"scrape_timeout_offset" json:"scrape_timeout_offset"` // offset to subtract from timeout in seconds
	MetricPrefix    string         `yaml:"metric_prefix" json:"metric_prefix"`                 // a prefix to ad dto all metric name; may be redefined in collector files
	QueryRetry      int            `yaml:"query_retry,omitempty" json:"query_retry,omitempty"` // target specific number of times to retry a query
	InvalidHttpCode any            `yaml:"invalid_auth_code,omitempty" json:"invalid_auth_code,omitempty"`
	ExporterName    string         `yaml:"exporter_name,omitempty" json:"exporter_name,omitempty"`

	UpMetricHelp        string `yaml:"up_help,omitempty" json:"up_help,omitempty"`
	ScrapeDurationHelp  string `yaml:"scrape_duration_help,omitempty" json:"scrape_duration_help,omitempty"`
	CollectorStatusHelp string `yaml:"collector_status_help,omitempty" json:"collector_status_help,omitempty"`
	QueryStatusHelp     string `yaml:"query_status_help,omitempty" json:"query_status_help,omitempty"`
	WebListenAddresses  string `yaml:"web.listen-address,omitempty" json:"web.listen-address,omitempty"`
	LogLevel            string `yaml:"log.level,omitempty" json:"log.level,omitempty"`
	TLSVersion          string `yaml:"tls_version,omitempty" json:"tls_version,omitempty"`

	invalid_auth_code []int
	tls_version       uint

	// query_retry int
	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for GlobalConfig.
func (g *GlobalConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Default to running the queries on every scrape.
	g.MinInterval = model.Duration(0)
	// Default to 10 seconds, since Prometheus has a 10 second scrape timeout default.
	g.ScrapeTimeout = model.Duration(10 * time.Second)
	// Default to .5 seconds.
	g.TimeoutOffset = model.Duration(500 * time.Millisecond)
	g.ExporterName = exporter_name
	g.UpMetricHelp = upMetricHelp
	g.ScrapeDurationHelp = scrapeDurationHelp
	g.CollectorStatusHelp = collectorStatusHelp
	g.QueryStatusHelp = queryStatusHelp
	g.tls_version = 0

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
	if g.ScrapeTimeout <= 0 {
		return fmt.Errorf("global.connection_timeout must be strictly positive, have %s", g.ScrapeTimeout)
	}

	if g.InvalidHttpCode == nil {
		g.invalid_auth_code = []int{401, 403}
	} else {
		g.invalid_auth_code = buildStatus(g.InvalidHttpCode)
	}

	if g.TLSVersion != "" {
		version := strings.ToLower(g.TLSVersion)

		if strings.Contains(version, "all") {
			g.tls_version = (tls_version_upto_1_2 | tls_version_1_2 | tls_version_1_3)
		} else {
			if strings.Contains(version, "tls_upto_1.2") {
				g.tls_version |= tls_version_upto_1_2
			}
			if strings.Contains(version, "tls_1.2") {
				g.tls_version |= tls_version_1_2
			}
			if strings.Contains(version, "tls_1.3") {
				g.tls_version |= tls_version_1_3
			}
		}
	}
	return checkOverflow(g.XXX, "global")
}

// *
// Targets
// *
const (
	TargetTypeStatic  = iota
	TargetTypeDynamic = iota
	TargetTypeModel   = iota
)

// TargetConfig defines a url and a set of collectors to be executed on it.
type TargetConfig struct {
	Name            string            `yaml:"name" json:"name"` // target name to connect to from prometheus
	Scheme          string            `yaml:"scheme" json:"scheme"`
	Host            string            `yaml:"host" json:"host"`
	Port            string            `yaml:"port,omitempty" json:"port,omitempty"`
	BaseUrl         string            `yaml:"baseUrl,omitempty" json:"baseUrl,omitempty"`
	AuthName        string            `yaml:"auth_name,omitempty" json:"auth_name,omitempty"`
	AuthConfig      AuthConfig        `yaml:"auth_config,omitempty" json:"auth_config,omitempty"`
	ProxyUrl        string            `yaml:"proxy,omitempty" json:"proxy,omitempty"`
	VerifySSLString string            `yaml:"verifySSL,omitempty" json:"verifySSL,omitempty"`
	ScrapeTimeout   model.Duration    `yaml:"scrape_timeout" json:"scrape_timeout"`                   // per-scrape timeout, global
	Labels          map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`               // labels to apply to all metrics collected from the targets
	CollectorRefs   []string          `yaml:"collectors" json:"collectors"`                           // names of collectors to execute on the target
	TargetsFiles    []string          `yaml:"targets_files,omitempty" json:"targets_files,omitempty"` // slice of path and pattern for files that contains targets
	QueryRetry      int               `yaml:"query_retry,omitempty" json:"query_retry,omitempty"`     // target specific number of times to retry a query
	ProfileName     string            `yaml:"profile" json:"profile"`

	collectors       []*CollectorConfig // resolved collector references
	fromFile         string             // filepath if loaded from targets_files pattern
	verifySSLUserSet bool
	verifySSL        ConvertibleBoolean
	targetType       int
	profile          *Profile

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
	// set default value for  VerifySSL
	t.verifySSL = true
	// by default value is not set by user; will be overwritten if user set a value
	t.verifySSLUserSet = false
	// default target type is static
	t.targetType = TargetTypeStatic
	// set profile to default
	t.ProfileName = "default"

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

		if strings.ToLower(t.Host) == "template" {
			t.targetType = TargetTypeModel
		} else {
			t.Host = check_env_var(t.Host)
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
		ProfileName:      t.ProfileName,
		CollectorRefs:    t.CollectorRefs,
		collectors:       t.collectors,
		verifySSLUserSet: t.verifySSLUserSet,
		verifySSL:        t.verifySSL,
		profile:          t.profile,
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

// CollectorConfig defines a set of metrics and how they are collected.
type CollectorConfig struct {
	Name           string                 `yaml:"collector_name" json:"collector_name"`                   // name of this collector
	MetricPrefix   string                 `yaml:"metric_prefix,omitempty" json:"metric_prefix,omitempty"` // a prefix to ad dto all metric name; may be redefined in collector files
	MinInterval    model.Duration         `yaml:"min_interval,omitempty" json:"min_interval,omitempty"`   // minimum interval between query executions
	Templates      map[string]string      `yaml:"templates,omitempty" json:"templates,omitempty"`         // share custom templates/funcs for results templating
	CollectScripts map[string]*YAMLScript `yaml:"scripts,omitempty" json:"scripts,omitempty"`             // map of all independent scripts to collect metrics - each script can run in parallem
	symtab         map[string]any

	customTemplate *exporterTemplate // to store the custom Templates used by this collector
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

	// build the default templates/funcs that my be used by all templates
	if len(c.Templates) > 0 {
		// c.customTemplate = template.New("default").Funcs(sprig.FuncMap())
		c.customTemplate = (*exporterTemplate)(template.New("default").Funcs(mytemplate.Mymap()))
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
	// build the js functions codes or required modules, from "jscode" to import
	// then in all Fields

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

type CollectorConfigList []*CollectorConfig

func (cs CollectorConfigList) String() string {
	buf := new(bytes.Buffer)
	buf.WriteString("[")

	for idx, c := range cs {
		if idx > 0 && idx <= len(cs)-1 {
			buf.WriteString(",")
		}
		buf.WriteString(c.Name)
	}
	buf.WriteString("]")
	return buf.String()
}

type dumpCollectorConfig struct {
	Name           string                 `yaml:"collector_name" json:"collector_name"`                   // name of this collector
	MetricPrefix   string                 `yaml:"metric_prefix,omitempty" json:"metric_prefix,omitempty"` // a prefix to ad dto all metric name; may be redefined in collector files
	MinInterval    model.Duration         `yaml:"min_interval,omitempty" json:"min_interval,omitempty"`   // minimum interval between query executions
	Templates      map[string]string      `yaml:"templates,omitempty" json:"templates,omitempty"`         // share custom templates/funcs for results templating
	CollectScripts map[string]ActionsList `yaml:"scripts,omitempty" json:"scripts,omitempty"`             // map of all independent scripts to collect metrics - each script can run in parallem
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

// MetricConfig defines a Prometheus metric, the SQL query to populate it and the mapping of columns to metric
// keys/values.
type MetricConfig struct {
	Name         string            `yaml:"metric_name" json:"metric_name"`                         // the Prometheus metric name
	TypeString   string            `yaml:"type" json:"type"`                                       // the Prometheus metric type
	Help         string            `yaml:"help" json:"help"`                                       // the Prometheus metric help text
	KeyLabels    any               `yaml:"key_labels,omitempty" json:"key_labels,omitempty"`       // expose these atributes as labels from JSON object: format name: value with name and value that should be template
	StaticLabels map[string]string `yaml:"static_labels,omitempty" json:"static_labels,omitempty"` // fixed key/value pairs as static labels
	ValueLabel   string            `yaml:"value_label,omitempty" json:"value_label,omitempty"`     // with multiple value columns, map their names under this label
	Values       map[string]string `yaml:"values" json:"values"`                                   // expose each of these columns as a value, keyed by column name
	Scope        string            `yaml:"scope,omitempty" json:"scope,omitempty"`                 // var path where to collect data: shortcut for {{ .scope.path.var }}

	valueType      prometheus.ValueType // TypeString converted to prometheus.ValueType
	name           *Field
	help           *Field
	key_labels_map map[string]string
	key_labels     *Field
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
	if name, err := NewField(m.Name, nil); err != nil {
		return err
	} else {
		m.name = name
	}

	if m.TypeString == "" {
		return fmt.Errorf("missing type for metric %q", m.Name)
	}

	if m.Help != "" {
		if help, err := NewField(m.Help, nil); err == nil {
			m.help = help
		} else {
			return err
		}
	}

	switch strings.ToLower(m.TypeString) {
	case "counter":
		m.valueType = prometheus.CounterValue
	case "gauge":
		m.valueType = prometheus.GaugeValue
	default:
		return fmt.Errorf("unsupported metric type: %s", m.TypeString)
	}

	// Check for duplicate key labels
	if m.KeyLabels != nil {
		switch ktype := m.KeyLabels.(type) {
		case map[string]string:
			for key, val := range ktype {
				if err := checkLabel(key, "metric", m.Name); err != nil {
					return err
				}
				// specific for format key_name: _ => replace by ${key_name}
				if val == "_" {
					ktype[key] = "$" + key
				}
			}
			m.key_labels_map = ktype
		case map[string]any:
			m.key_labels_map = make(map[string]string, len(ktype))
			for key, val_raw := range ktype {
				if err := checkLabel(key, "metric", m.Name); err != nil {
					return err
				}
				if val, ok := val_raw.(string); ok {
					if val == "_" {
						val = "$" + key
					}
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

// MarshalJSON implements the json.Marshaler interface for Secrets.
func (s Secret) MarshalJSON() ([]byte, error) {
	if s != "" {
		return []byte("\"<secret>\""), nil
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
	Mode        string             `yaml:"mode,omitempty" json:"mode,omitempty"` // basic, encrypted, bearer
	Username    string             `yaml:"user,omitempty" json:"user,omitempty"`
	Password    Secret             `yaml:"password,omitempty" json:"password,omitempty"`
	Token       Secret             `yaml:"token,omitempty" json:"token,omitempty"`
	DisableWarn ConvertibleBoolean `yaml:"disable_warn,omitempty" json:"disable_warn,omitempty"`

	authKey string
}

func check_env_var(value string) string {
	if value != "" && strings.HasPrefix(value, "$env:") {
		value = os.Getenv(value[5:])
	}
	return value
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for authConfig
func (auth *AuthConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// auth.DisableWarn = false
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
			pat := regexp.MustCompile(strings.TrimSpace(cref[1:]))
			for c_name, c := range collectors {
				if pat.MatchString(c_name) {
					resolved = append(resolved, c)
				}
			}
		} else if strings.HasPrefix(cref, "!~") {
			pat := regexp.MustCompile(strings.TrimSpace(cref[2:]))
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
