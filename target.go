package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

const (
	// Capacity for the channel to collect metrics.
	capMetricChan = 1000

	// Capacity for the channel to collect control message from collectors.
	capCollectChan = 100

	upMetricName       = "up"
	upMetricHelp       = "1 if the target is reachable, or 0 if the scrape failed"
	scrapeDurationName = "scrape_duration_seconds"
	scrapeDurationHelp = "How long it took to scrape the target in seconds"
)

// Target collects SQL metrics from a single sql.DB instance. It aggregates one or more Collectors and it looks much
// like a prometheus.Collector, except its Collect() method takes a Context to run in.
type Target interface {
	// Collect is the equivalent of prometheus.Collector.Collect(), but takes a context to run in.
	Collect(ctx context.Context, ch chan<- Metric)
	Name() string
	SetSymbol(string, any) error
	// YAML() ([]byte, error)
}

// target implements Target. It wraps a httpAPI, which is initially nil but never changes once instantianted.
type target struct {
	name               string
	client             *Client
	collectors         []Collector
	constLabels        prometheus.Labels
	globalConfig       *GlobalConfig
	httpAPIScript      map[string]*YAMLScript
	upDesc             MetricDesc
	scrapeDurationDesc MetricDesc
	logContext         []interface{}

	logger log.Logger

	// symbols table
	// tsymtab map[string]any

	// mutex to hold condition for global client to try to login()
	// wait_mutex sync.Mutex
	// wake_cond  sync.Cond

	// to protect the data during exchange
	// content_mutex sync.Mutex
	// msg           int
}

const (
	// one collector needs a ping action
	MsgPing = iota
	// one collector needs a login action
	MsgLogin = iota
	// start-restart collect for all collectors
	MsgCollect = iota
	// wait for all collectors to reply
	MsgWait = iota
	// collecting is over
	MsgQuit = iota
	// one collect is over
	MsgDone = iota
)

// NewTarget returns a new Target with the given instance name, data source name, collectors and constant labels.
// An empty target name means the exporter is running in single target mode: no synthetic metrics will be exported.
func NewTarget(
	logContext []interface{},
	tpar *TargetConfig,
	ccs []*CollectorConfig,
	constLabels prometheus.Labels,
	gc *GlobalConfig,
	http_script map[string]*YAMLScript,
	logger log.Logger) (Target, error) {

	if tpar.Name != "" {
		logContext = append(logContext, "target", tpar.Name)
	}

	constLabelPairs := make([]*dto.LabelPair, 0, len(constLabels))
	for n, v := range constLabels {
		constLabelPairs = append(constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(constLabelPairs))

	collectors := make([]Collector, 0, len(ccs))
	for _, cc := range ccs {
		cscrl := make([]*YAMLScript, len(cc.CollectScripts))
		i := 0
		for _, cs := range cc.CollectScripts {
			cscrl[i] = cs
			i++
		}
		c, err := NewCollector(logContext, logger, cc, constLabelPairs, cscrl)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, c)
	}

	upDesc := NewAutomaticMetricDesc(logContext, gc.MetricPrefix+"_"+upMetricName, upMetricHelp, prometheus.GaugeValue, constLabelPairs)
	scrapeDurationDesc :=
		NewAutomaticMetricDesc(logContext, gc.MetricPrefix+"_"+scrapeDurationName, scrapeDurationHelp, prometheus.GaugeValue, constLabelPairs)

	t := target{
		name:               tpar.Name,
		client:             newClient(tpar, http_script, logger, gc),
		collectors:         collectors,
		constLabels:        constLabels,
		globalConfig:       gc,
		httpAPIScript:      http_script,
		upDesc:             upDesc,
		scrapeDurationDesc: scrapeDurationDesc,
		logContext:         logContext,
		logger:             logger,
		// wait_mutex:         sync.Mutex{},
		// content_mutex:      sync.Mutex{},
	}
	if t.client == nil {
		return nil, fmt.Errorf("internal http client undefined")
	}
	// t.wake_cond = *sync.NewCond(&t.wait_mutex)

	// init the symbols tab
	// t.symtab = make(map[string]any)

	return &t, nil
}

// Name implement Target.Name
// to obtain target name from interface
func (t *target) Name() string {
	return t.name
}

// SetSymbol implement Target.SetSymbol
func (t *target) SetSymbol(key string, value any) error {

	t.client.symtab[key] = value
	return nil
}

// func (t *target) startCollect(wg *sync.WaitGroup, ctx context.Context, ch chan<- Metric) {
// 	wg.Add(len(t.collectors))
// 	for _, c := range t.collectors {
// 		// have to build a new client copy to allow multi connection to target
// 		c_client := t.client.Clone()
// 		c.SetClient(c_client)

// 		// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
// 		go func(collector Collector) {
// 			defer wg.Done()
// 			collector.Collect(ctx, ch)
// 		}(c)
// 	}
// }

// Collect implements Target.
func (t *target) Collect(ctx context.Context, ch chan<- Metric) {

	// chan to receive order from collector if something wrong with authentication
	collectChan := make(chan int, capCollectChan)

	var (
		scrapeStart = time.Now()
		wg_coll     sync.WaitGroup
		targetUp    bool
		err         error
	)

	// wait for all collectors are over
	defer func() {
		wg_coll.Wait()
	}()

	// to store is we already have tried to login
	has_logged := false

	//	t.client.client.httpClient.Transport.TLSClientConfig.InsecureSkipVerify
	// try to connect to target
	collectChan <- MsgPing

	leave_loop := false
	for msg := range collectChan {
		// t.content_mutex.Lock()
		// msg := t.msg
		// t.content_mutex.Unlock()
		switch msg {

		case MsgLogin:
			level.Debug(t.logger).Log("msg", "target: received MsgLogin")
			if msg == MsgLogin && !has_logged {
				t.client.Clear()
				if status, err := t.client.Login(); err != nil {
					collectChan <- MsgQuit
				} else {
					if status {
						has_logged = true
						collectChan <- MsgPing
					} else {
						collectChan <- MsgQuit
					}
				}
			}
		case MsgPing:
			level.Debug(t.logger).Log("msg", "target: received MsgPing")
			// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
			wg_coll.Add(1)
			go func(t *target, ch chan<- Metric, coll_ch chan<- int) {
				defer wg_coll.Done()
				// collector.Collect(ctx, met_ch, &t.wake_cond)
				targetUp, err = t.ping(ctx, ch, collectChan)
			}(t, ch, collectChan)
			level.Debug(t.logger).Log("msg", "target: ping send MsgWait")
			collectChan <- MsgWait

		case MsgQuit:
			level.Debug(t.logger).Log("msg", "target: ping received MsgQuit")
			leave_loop = true

		case MsgWait:
			level.Debug(t.logger).Log("msg", "start waiting for ping is over")
			wg_coll.Wait()
			level.Debug(t.logger).Log("msg", "after waiting for ping is over")
			need_login := false
			submsg := <-collectChan

			switch submsg {
			case MsgLogin:
				level.Debug(t.logger).Log("msg", "target ping wait: received MsgLogin")
				need_login = true
			case MsgDone:
				level.Debug(t.logger).Log("msg", "target ping wait: received MsgDone")
			default:
				level.Debug(t.logger).Log("msg", fmt.Sprintf("target ping wait: received msg [%d]", submsg))
			}
			if need_login {
				collectChan <- MsgLogin
			} else {
				collectChan <- MsgQuit
			}
		}
		// leave collectChan loop
		if leave_loop {
			break
		}
	}

	// targetUp, err := t.ping(ctx)
	if err != nil {
		ch <- NewInvalidMetric(t.logContext, err)
		targetUp = false
	}
	if t.name != "" {
		// Export the target's `up` metric as early as we know what it should be.
		ch <- NewMetric(t.upDesc, boolToFloat64(targetUp), nil, nil)
	}

	// Don't bother with the collectors if target is down.
	if targetUp {
		// var wg sync.WaitGroup

		// mutex to hold condition for global client to try to login()
		// wait_mutex := sync.Mutex{}
		// wake_cond := *sync.NewCond(&t.wait_mutex)

		// wg.Add(1)
		// go func(t *target, ctx context.Context, met_ch chan<- Metric) {

		// ok := true
		has_logged := false
		// check if we have already logged to target ?
		// if not then set msg to directly try to login
		if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
			has_logged = logged
		}

		if has_logged {
			// if already logged only start to collect metrics
			collectChan <- MsgCollect
		} else {
			// else not already logged so start to login in
			collectChan <- MsgLogin
		}

		for msg := range collectChan {
			// t.content_mutex.Lock()
			// msg := t.msg
			// t.content_mutex.Unlock()
			switch msg {

			case MsgLogin:
				level.Debug(t.logger).Log("msg", "target: received MsgLogin")
				// check if we have already logged to target ?
				// if not then set msg to directly try to login
				// if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
				// 	// t.content_mutex.Lock()
				// 	// t.msg = MsgLogin
				// 	// t.content_mutex.Unlock()
				// 	has_logged = logged
				// }
				level.Debug(t.logger).Log("msg", fmt.Sprintf("target: MsgLogin check has_logged: %v", has_logged))
				if msg == MsgLogin && !has_logged {
					//
					// wait until all collectors have finished
					wg_coll.Wait()
					t.client.Clear()
					t.client.symtab["__coll_channel"] = collectChan

					if status, err := t.client.Login(); err != nil {
						collectChan <- MsgQuit
					} else {
						if status {
							has_logged = true
							collectChan <- MsgCollect
							// t.msg = MsgEmpty
							// msg = MsgEmpty
						} else {
							collectChan <- MsgQuit

							// t.msg = MsgQuit
							// msg = MsgQuit
						}
					}
					delete(t.client.symtab, "__coll_channel")
				}
			case MsgCollect:
				level.Debug(t.logger).Log("msg", "target: received MsgCollect")
				wg_coll.Add(len(t.collectors))
				level.Debug(t.logger).Log("msg", fmt.Sprintf("target: send %d collector(s)", len(t.collectors)))
				for _, c := range t.collectors {
					// have to build a new client copy to allow multi connection to target
					c_client := t.client.Clone()
					c.SetClient(c_client)
					// c.SetClient(t.client)

					// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
					go func(collector Collector) {
						defer wg_coll.Done()
						// collector.Collect(ctx, met_ch, &t.wake_cond)
						collector.Collect(ctx, ch, collectChan)
					}(c)
				}
				level.Debug(t.logger).Log("msg", "target: send MsgWait")
				collectChan <- MsgWait

			case MsgQuit:
				level.Debug(t.logger).Log("msg", "target: received MsgQuit")
				close(collectChan)

			case MsgWait:
				level.Debug(t.logger).Log("msg", "start waiting for all collectors are over")

				wg_coll.Wait()
				level.Debug(t.logger).Log("msg", "after waiting for all collectors is over")
				need_login := false
				// we have received len(t.collectors) msgs from collectors
				for i := 0; i < len(t.collectors); i++ {
					level.Debug(t.logger).Log("msg", fmt.Sprintf("target wait: read msg from collector %d", i))
					submsg := <-collectChan
					// for submsg := range collectChan {

					switch submsg {
					case MsgLogin:
						level.Debug(t.logger).Log("msg", "target wait: received MsgLogin")
						need_login = true
					default:
						level.Debug(t.logger).Log("msg", fmt.Sprintf("target wait: received msg [%d] for collector %d", submsg, i))
					}
				}
				if need_login {
					collectChan <- MsgLogin
					t.client.symtab["logged"] = false
					has_logged = false
					if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
						level.Debug(t.logger).Log("msg", fmt.Sprintf("target: MsgLogin check has_logged: %v", logged))
					}
				} else {
					collectChan <- MsgQuit
				}
			}
			// level.Debug(t.logger).Log("msg", "goroutine login() is waiting")
			// t.wait_mutex.Lock()
			// t.wake_cond.Wait()
		}
		level.Debug(t.logger).Log("msg", "goroutine target collector controler is over")
		// }(t, ctx, ch)

		// wait_mutex.Lock()
		// wake_cond.Wait()
		// Wait for all collectors to complete, then close the channel.
		// go func() {
		// 	wg.Wait()
		// 	close(collectChan)
		// }()

		// Drain collectChan in case of premature return.
		defer func() {
			for range collectChan {
			}
		}()
	}

	// Wait for all collectors (if any) to complete.
	// wg.Wait()
	level.Debug(t.logger).Log("msg", "collectors have stopped")
	// tell the "waiting login" func to stop
	// t.content_mutex.Lock()
	// t.msg = MsgQuit
	// t.content_mutex.Unlock()
	// t.wake_cond.Signal()

	if t.name != "" {
		// And export a `scrape duration` metric once we're done scraping.
		ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil, nil)
	}
}

// YAML marshals the config into YAML format.
// type tconfig struct {
// 	Name       string     `yaml:"name"` // target name to connect to from prometheus
// 	Scheme     string     `yaml:"scheme"`
// 	Host       string     `yaml:"host"`
// 	Port       string     `yaml:"port"`
// 	BaseUrl    string     `yaml:"baseUrl"`
// 	AuthConfig AuthConfig `yaml:"auth_mode,omitempty"`
// 	ProxyUrl          string             `yaml:"proxy"`
// 	VerifySSL         ConvertibleBoolean `yaml:"verifySSL"`
// 	ConnectionTimeout model.Duration     `yaml:"connection_timeout"`      // connection timeout, per-target
// 	Labels            map[string]string  `yaml:"labels,omitempty"`        // labels to apply to all metrics collected from the targets
// 	CollectorRefs     []string           `yaml:"collectors"`              // names of collectors to execute on the target
// 	TargetsFiles      []string           `yaml:"targets_files,omitempty"` // slice of path and pattern for files that contains targets
// 	QueryRetry        string             `yaml:"query_retry,omitempty"`   // target specific number of times to retry a query
// }

// func (t *target) YAML() ([]byte, error) {
// 	tcfg := &tconfig{
// 		Name:              t.Name(),
// 		Scheme:            t.Sc,
// 		Host:              "",
// 		Port:              "",
// 		BaseUrl:           "",
// 		AuthConfig:        AuthConfig{},
// 		ProxyUrl:          "",
// 		VerifySSL:         false,
// 		ConnectionTimeout: t,
// 		Labels:            map[string]string{},
// 		CollectorRefs:     []string{},
// 		TargetsFiles:      []string{},
// 		QueryRetry:        "",
// 	}
// 	return yaml.Marshal(tcfg)
// }

// ping implement ping for target
func (t *target) ping(ctx context.Context, ch chan<- Metric, coll_ch chan<- int) (bool, error) {
	t.client.symtab["__coll_channel"] = coll_ch
	status, err := t.client.Ping()
	delete(t.client.symtab, "__coll_channel")
	return status, err
}

// boolToFloat64 converts a boolean flag to a float64 value (0.0 or 1.0).
func boolToFloat64(value bool) float64 {
	if value {
		return 1.0
	}
	return 0.0
}

// type targets []target

// func (ts targets) YAML() ([]byte, error) {
// 	var full, content []byte
// 	var err error

// 	for t := range ts {
// 		content, err = yaml.Marshal(t)
// 		if err != nil {
// 			content = nil
// 			return content, err
// 		}
// 		full = append(full, content...)
// 	}
// 	return full, err
// }

// type Targets interface {
// 	YAML() ([]byte, error)
// }

// func (ts *Targets) YAML() ([]byte, error) {
// 	var full, content []byte
// 	var err error

// 	for t := range *ts {
// 		content, err = yaml.Marshal(t)
// 		if err != nil {
// 			content = nil
// 			return content, err
// 		}
// 		full = append(full, content...)
// 	}
// 	return full, err
// }
