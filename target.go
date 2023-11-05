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

	upMetricName        = "up"
	upMetricHelp        = "1 if the target is reachable, or 0 if the scrape failed"
	scrapeDurationName  = "scrape_duration_seconds"
	scrapeDurationHelp  = "How long it took to scrape the target in seconds"
	collectorStatusName = "collector_status"
	collectorStatusHelp = "collector scripts status 0: error - 1: ok"
)

// Target collects SQL metrics from a single sql.DB instance. It aggregates one or more Collectors and it looks much
// like a prometheus.Collector, except its Collect() method takes a Context to run in.
type Target interface {
	// Collect is the equivalent of prometheus.Collector.Collect(), but takes a context to run in.
	Collect(ctx context.Context, ch chan<- Metric)
	Name() string
	SetSymbol(string, any) error
	Config() *TargetConfig
	GetDeadline() time.Time
	SetDeadline(time.Time)
	SetLogger(log.Logger)
	// Update(string, string) error
	// YAML() ([]byte, error)
}

// target implements Target. It wraps a httpAPI, which is initially nil but never changes once instantianted.
type target struct {
	name        string
	config      *TargetConfig
	client      *Client
	collectors  []Collector
	constLabels prometheus.Labels
	// globalConfig       *GlobalConfig
	httpAPIScript       map[string]*YAMLScript
	upDesc              MetricDesc
	scrapeDurationDesc  MetricDesc
	collectorStatusDesc MetricDesc
	logContext          []interface{}

	logger   log.Logger
	deadline time.Time

	// symbols table
	// tsymtab map[string]any

	// mutex to hold condition for global client to try to login()
	// wait_mutex sync.Mutex
	// wake_cond  sync.Cond

	has_ever_logged bool
	// to protect the data during exchange
	content_mutex *sync.Mutex
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
	// one collect has received global timeout
	MsgTimeout = iota
)

func Msg2Text(msg int) string {
	var ret string
	switch msg {
	case MsgPing:
		ret = "MsgPing"
	case MsgLogin:
		ret = "MsgLogin"
	case MsgCollect:
		ret = "MsgCollect"
	case MsgWait:
		ret = "MsgWait"
	case MsgQuit:
		ret = "MsgQuit"
	case MsgDone:
		ret = "MsgDone"
	case MsgTimeout:
		ret = "MsgTimeout"
	}
	return ret
}

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

	collectorStatusDesc := NewAutomaticMetricDesc(logContext,
		gc.MetricPrefix+"_"+collectorStatusName,
		collectorStatusHelp,
		prometheus.GaugeValue, constLabelPairs,
		"collectorname")

	t := target{
		name:        tpar.Name,
		config:      tpar,
		client:      newClient(tpar, http_script, logger, gc),
		collectors:  collectors,
		constLabels: constLabels,
		// globalConfig:       gc,
		httpAPIScript:       http_script,
		upDesc:              upDesc,
		scrapeDurationDesc:  scrapeDurationDesc,
		collectorStatusDesc: collectorStatusDesc,
		logContext:          logContext,
		logger:              logger,
		// wait_mutex:         sync.Mutex{},
		content_mutex: &sync.Mutex{},
	}
	if t.client == nil {
		return nil, fmt.Errorf("internal http client undefined")
	}
	// shared content mutex between target and client
	t.client.content_mutex = t.content_mutex

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

// Config implement Target.Name for target
// to obtain target name from interface
func (t *target) Config() *TargetConfig {
	return t.config
}

// SetSymbol implement Target.SetSymbol
func (t *target) SetSymbol(key string, value any) error {

	t.client.symtab[key] = value
	return nil
}

// Getter for deadline
func (t *target) GetDeadline() time.Time {
	return t.deadline
}

// Setter for deadline
func (t *target) SetDeadline(tt time.Time) {
	t.deadline = tt
}

func (t *target) SetLogger(logger log.Logger) {
	t.content_mutex.Lock()
	t.logger = logger
	t.client.logger = logger
	for _, c := range t.collectors {
		c.SetLogger(logger)
	}
	t.content_mutex.Unlock()
}

// func (t *target) Update(host_path string, auth_name string) error {
// 	url_elmt, err := url.Parse(host_path)
// 	if err != nil {
// 		return err
// 	}
// 	if t.config.Scheme != url_elmt.Scheme {
// 		t.config.Scheme = url_elmt.Scheme
// 	}
// 	elmts := strings.Split(url_elmt.Host, ":")
// 	if t.config.Host != elmts[0] {
// 		t.config.Host = elmts[0]
// 	}
// 	if len(elmts) > 1 {
// 		t.config.Port = elmts[1]
// 	}
// 	if url_elmt.User.Username() != "" {
// 		t.config.AuthConfig.Username = url_elmt.User.Username()
// 		if tmp, set := url_elmt.User.Password(); set {
// 			t.config.AuthConfig.Password = Secret(tmp)
// 		}
// 		t.config.AuthConfig.Mode = "basic"
// 	}
// 	return nil
// }

// Collect implements Target.
func (t *target) Collect(ctx context.Context, met_ch chan<- Metric) {

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
	t.client.symtab["__collector_id"] = t.name
	for msg := range collectChan {
		// t.content_mutex.Lock()
		// msg := t.msg
		// t.content_mutex.Unlock()
		switch msg {

		case MsgLogin:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "target: received MsgLogin")
			if msg == MsgLogin && !has_logged {
				t.client.Clear()
				if status, err := t.client.Login(); err != nil {
					level.Error(t.logger).Log(
						"collid", fmt.Sprintf("ping/%s", t.name),
						"script", "ping/login",
						"errmsg", err)
					collectChan <- MsgQuit
				} else {
					if status {
						has_logged = true
						t.has_ever_logged = true
						collectChan <- MsgPing
					} else {
						collectChan <- MsgQuit
					}
				}
			}
		case MsgPing:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "target: received MsgPing")
			// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
			wg_coll.Add(1)
			go func(t *target, met_ch chan<- Metric, coll_ch chan<- int) {
				defer wg_coll.Done()
				// collector.Collect(ctx, met_ch, &t.wake_cond)
				targetUp, err = t.ping(ctx, met_ch, collectChan)
			}(t, met_ch, collectChan)
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "target: ping send MsgWait")
			collectChan <- MsgWait

		case MsgQuit:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "target: ping received MsgQuit")
			leave_loop = true

		case MsgTimeout:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", t.name,
				"msg", "target: ping received MsgTimeout")
			leave_loop = true

		case MsgWait:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "start waiting for ping is over")
			wg_coll.Wait()
			t.content_mutex.Lock()
			logger = t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"collid", fmt.Sprintf("ping/%s", t.name),
				"msg", "after waiting for ping is over")
			need_login := false
			submsg := <-collectChan

			switch submsg {
			case MsgLogin:
				level.Debug(logger).Log(
					"collid", fmt.Sprintf("ping/%s", t.name),
					"msg", "target ping wait: received MsgLogin")
				need_login = true
			case MsgDone:
				level.Debug(logger).Log(
					"collid", fmt.Sprintf("ping/%s", t.name),
					"msg", "target ping wait: received MsgDone")
			default:
				level.Debug(logger).Log(
					"collid", fmt.Sprintf("ping/%s", t.name),
					"msg", fmt.Sprintf("target ping wait: received msg =[%s] from collector", Msg2Text(submsg)))
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
	delete(t.client.symtab, "__collector_id")

	// targetUp, err := t.ping(ctx)
	if err != nil {
		met_ch <- NewInvalidMetric(t.logContext, err)
		targetUp = false
	}
	if t.name != "" {
		// Export the target's `up` metric as early as we know what it should be.
		met_ch <- NewMetric(t.upDesc, boolToFloat64(targetUp), nil, nil)
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

		// to check if channel is already closed
		has_quitted := false

		for msg := range collectChan {
			// t.content_mutex.Lock()
			// msg := t.msg
			// t.content_mutex.Unlock()
			switch msg {

			case MsgLogin:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", fmt.Sprintf("target: received MsgLogin / check has_logged: %v", has_logged))
				// check if we have already logged to target ?
				// if not then set msg to directly try to login
				// if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
				// 	// t.content_mutex.Lock()
				// 	// t.msg = MsgLogin
				// 	// t.content_mutex.Unlock()
				// 	has_logged = logged
				// }
				t.client.symtab["__collector_id"] = t.name
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
				delete(t.client.symtab, "__collector_id")

			case MsgCollect:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "target: received MsgCollect")
				wg_coll.Add(len(t.collectors))
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", fmt.Sprintf("target: send %d collector(s)", len(t.collectors)))

				t.client.symtab["__coll_channel"] = collectChan

				for _, c := range t.collectors {
					t.client.symtab["__collector_id"] = fmt.Sprintf("%s#%s", t.name, c.GetId())
					// have to build a new client copy to allow multi connection to target
					c_client := t.client.Clone(t.config)
					c.SetClient(c_client)
					// c.SetClient(t.client)

					// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
					go func(collector Collector, deadline time.Time) {
						var (
							coll_ctx context.Context
							cancel   context.CancelFunc
						)
						// init a cancelable calling  for each collector if a deadline has been set for scraping
						if deadline.IsZero() {
							coll_ctx = ctx
							cancel = func() {
							}
						} else {
							coll_ctx, cancel = context.WithDeadline(ctx, deadline)
						}

						defer func() {
							cancel()
							wg_coll.Done()
						}()
						// collector.Collect(ctx, met_ch, &t.wake_cond)
						collector.Collect(coll_ctx, met_ch, collectChan)
					}(c, t.GetDeadline())
				}
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "target: send MsgWait")
				delete(t.client.symtab, "__collector_id")

				collectChan <- MsgWait

			case MsgTimeout:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "target: received MsgTimeout")

				if !has_quitted {
					close(collectChan)
					has_quitted = true
				}

			case MsgQuit:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "target: received MsgQuit")

				if !has_quitted {
					close(collectChan)
					has_quitted = true
				}

			case MsgWait:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "start waiting for all collectors are over")

				wg_coll.Wait()
				t.content_mutex.Lock()
				logger = t.logger
				t.content_mutex.Unlock()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", "after waiting for all collectors is over")
				need_login := false
				// we have received len(t.collectors) msgs from collectors
				for i := 0; i < len(t.collectors); i++ {
					level.Debug(logger).Log(
						"collid", t.name,
						"msg", fmt.Sprintf("target wait: read msg[%d] from collector", i))
					submsg := <-collectChan
					// for submsg := range collectChan {

					switch submsg {
					case MsgLogin:
						level.Debug(logger).Log(
							"collid", t.name,
							"msg", "target wait: received MsgLogin")
						need_login = true
					default:
						level.Debug(logger).Log(
							"collid", t.name,
							"msg", fmt.Sprintf("target wait: received msg[%d]=[%s] from collector", i, Msg2Text(submsg)))
					}
				}
				if need_login {
					collectChan <- MsgLogin
					t.client.symtab["logged"] = false
					has_logged = false
					if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
						level.Debug(logger).Log(
							"collid", t.name,
							"msg", fmt.Sprintf("target: MsgLogin check has_logged: %v", logged))
					}
				} else {
					collectChan <- MsgQuit
				}
			}
			// level.Debug(t.logger).Log("msg", "goroutine login() is waiting")
			// t.wait_mutex.Lock()
			// t.wake_cond.Wait()
		}
		level.Debug(t.logger).Log(
			"collid", t.name,
			"msg", "goroutine target collector controler is over")
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
			delete(t.client.symtab, "__coll_channel")
			delete(t.client.symtab, "__collector_id")
		}()
	}

	t.content_mutex.Lock()
	logger := t.logger
	t.content_mutex.Unlock()
	level.Debug(logger).Log("msg", "collectors have stopped")

	if t.name != "" {
		// And exporter a `collector execution status` metric for each collector once we're done scraping.
		if targetUp {
			labels_name := make([]string, 1)
			labels_name[0] = "collectorname"
			labels_value := make([]string, 1)
			for _, c := range t.collectors {
				labels_value[0] = c.GetName()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", fmt.Sprintf("target collector['%s'] collid=[%s] has status [%d]", labels_value[0], c.GetId(), c.GetStatus()))
				met_ch <- NewMetric(t.collectorStatusDesc, float64(c.GetStatus()), labels_name, labels_value)
			}
		}
		// And exporter a `scrape duration` metric once we're done scraping.
		met_ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil, nil)
	}
}

// ping implement ping for target
func (t *target) ping(ctx context.Context, ch chan<- Metric, coll_ch chan<- int) (bool, error) {
	t.client.symtab["__coll_channel"] = coll_ch
	// t.client.symtab["__collector_id"] = t.name
	status, err := t.client.Ping()
	if err != nil {
		switch err {
		case ErrInvalidLogin:
			if t.has_ever_logged {
				level.Error(t.logger).Log(
					"collid", fmt.Sprintf("ping/%s", t.name),
					"errmsg", err)
			}
			coll_ch <- MsgLogin
		case ErrContextDeadLineExceeded:
			coll_ch <- MsgTimeout
		default:
			// level.Warn(t.logger).Log(
			// 	"collid", CollectorId(t.client.symtab, t.logger),
			// 	"script", ScriptName(t.client.symtab, t.logger),
			// 	"errmsg", err)
			coll_ch <- MsgQuit
		}
	} else {
		coll_ch <- MsgDone
	}
	delete(t.client.symtab, "__coll_channel")
	// delete(t.client.symtab, "__collector_id")
	return status, err
}

// boolToFloat64 converts a boolean flag to a float64 value (0.0 or 1.0).
func boolToFloat64(value bool) float64 {
	if value {
		return 1.0
	}
	return 0.0
}
