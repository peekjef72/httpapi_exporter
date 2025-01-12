package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/imdario/mergo"
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
	collectorStatusHelp = "collector scripts status 0: error - 1: ok - 2: Invalid login 3: Timeout"
)

// Target collects SQL metrics from a single sql.DB instance. It aggregates one or more Collectors and it looks much
// like a prometheus.Collector, except its Collect() method takes a Context to run in.
type Target interface {
	// Collect is the equivalent of prometheus.Collector.Collect(), but takes a context to run in.
	Collect(ctx context.Context, ch chan<- Metric, health_only bool)
	Name() string
	SetSymbol(string, any) error
	GetSymbolTable() map[string]any
	Config() *TargetConfig
	GetDeadline() time.Time
	SetDeadline(time.Time)
	SetLogger(*slog.Logger)
	Lock()
	Unlock()
}

// target implements Target. It wraps a httpAPI, which is initially nil but never changes once instantianted.
type target struct {
	name                string
	config              *TargetConfig
	client              *Client
	collectors          []Collector
	httpAPIScript       map[string]*YAMLScript
	upDesc              MetricDesc
	scrapeDurationDesc  MetricDesc
	collectorStatusDesc MetricDesc
	logContext          []interface{}

	logger   *slog.Logger
	deadline time.Time

	has_ever_logged bool
	// to protect the data during exchange
	content_mutex *sync.Mutex
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
	gc *GlobalConfig,
	profile *Profile,
	logger *slog.Logger) (Target, error) {

	if tpar.Name != "" {
		logContext = append(logContext, "target", tpar.Name)
	}

	constLabelPairs := make([]*dto.LabelPair, 0, len(tpar.Labels))
	for n, v := range tpar.Labels {
		constLabelPairs = append(constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(constLabelPairs))

	collectors := make([]Collector, 0, len(tpar.collectors))
	for _, cc := range tpar.collectors {
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

	upDesc := NewAutomaticMetricDesc(logContext, profile.MetricPrefix+"_"+upMetricName, gc.UpMetricHelp, prometheus.GaugeValue, constLabelPairs)
	scrapeDurationDesc :=
		NewAutomaticMetricDesc(logContext, profile.MetricPrefix+"_"+scrapeDurationName, gc.ScrapeDurationHelp, prometheus.GaugeValue, constLabelPairs)

	collectorStatusDesc := NewAutomaticMetricDesc(logContext,
		profile.MetricPrefix+"_"+collectorStatusName,
		gc.CollectorStatusHelp,
		prometheus.GaugeValue, constLabelPairs,
		"collectorname")

	t := target{
		name:                tpar.Name,
		config:              tpar,
		client:              newClient(tpar, profile.Scripts, logger, gc),
		collectors:          collectors,
		httpAPIScript:       profile.Scripts,
		upDesc:              upDesc,
		scrapeDurationDesc:  scrapeDurationDesc,
		collectorStatusDesc: collectorStatusDesc,
		logContext:          logContext,
		logger:              logger,
		content_mutex:       &sync.Mutex{},
	}
	if t.client == nil {
		return nil, errors.New("internal http client undefined")
	}
	// shared content mutex between target and client
	t.client.content_mutex = t.content_mutex

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
//
// add or update element in symbol table
//
// May be unitary key (.attribute) or sequence (.attr1.attr2.[...])
func (t *target) SetSymbol(key string, value any) error {
	symtab := t.client.symtab
	if r_val, ok := symtab[key]; ok {
		vDst := reflect.ValueOf(r_val)
		if vDst.Kind() == reflect.Map {
			if m_val, ok := r_val.(map[string]any); ok {
				opts := mergo.WithOverride
				if err := mergo.Merge(&m_val, value, opts); err != nil {
					return err
				}
			}
		} else if vDst.Kind() == reflect.Slice {
			if s_val, ok := r_val.([]any); ok {
				opts := mergo.WithOverride
				if err := mergo.Merge(&s_val, value, opts); err != nil {
					return err
				}
			}
		} else {
			symtab[key] = value
		}
	} else {
		symtab[key] = value
	}
	return nil
}

func (t *target) GetSymbolTable() map[string]any {
	return t.client.symtab
}

// Getter for deadline
func (t *target) GetDeadline() time.Time {
	return t.deadline
}

// Setter for deadline
func (t *target) SetDeadline(tt time.Time) {
	t.deadline = tt
}

func (t *target) SetLogger(logger *slog.Logger) {
	t.content_mutex.Lock()
	t.logger = logger
	t.client.logger = logger
	for _, c := range t.collectors {
		c.SetLogger(logger)
	}
	t.content_mutex.Unlock()
}

func (t *target) Lock() {
	t.content_mutex.Lock()
}

func (t *target) Unlock() {
	t.content_mutex.Unlock()
}

// Collect implements Target.
func (t *target) Collect(ctx context.Context, met_ch chan<- Metric, health_only bool) {

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

	// try to connect to target
	collectChan <- MsgPing

	leave_loop := false
	t.client.symtab["__collector_id"] = t.name
	for msg := range collectChan {
		switch msg {

		case MsgLogin:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"target: received MsgLogin",
				"collid", fmt.Sprintf("ping/%s", t.name))
			if msg == MsgLogin && !has_logged {
				t.client.Clear()
				if status, err := t.client.Login(); err != nil {
					t.logger.Error(
						err.Error(),
						"collid", fmt.Sprintf("ping/%s", t.name),
						"script", "ping/login")
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
			logger.Debug(
				"target: received MsgPing",
				"collid", fmt.Sprintf("ping/%s", t.name))
			// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
			wg_coll.Add(1)
			go func(t *target, met_ch chan<- Metric, coll_ch chan<- int) {
				defer wg_coll.Done()
				targetUp, err = t.ping(collectChan)
			}(t, met_ch, collectChan)
			logger.Debug(
				"target: ping send MsgWait",
				"collid", fmt.Sprintf("ping/%s", t.name))
			collectChan <- MsgWait

		case MsgQuit:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"target: ping received MsgQuit",
				"collid", fmt.Sprintf("ping/%s", t.name))
			leave_loop = true

		case MsgTimeout:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"target: ping received MsgTimeout",
				"collid", t.name)
			leave_loop = true

		case MsgWait:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"start waiting for ping is over",
				"collid", fmt.Sprintf("ping/%s", t.name))
			wg_coll.Wait()
			t.content_mutex.Lock()
			logger = t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"after waiting for ping is over",
				"collid", fmt.Sprintf("ping/%s", t.name))
			need_login := false
			submsg := <-collectChan

			switch submsg {
			case MsgLogin:
				logger.Debug(
					"target ping wait: received MsgLogin",
					"collid", fmt.Sprintf("ping/%s", t.name))
				need_login = true
			case MsgDone:
				logger.Debug(
					"target ping wait: received MsgDone",
					"collid", fmt.Sprintf("ping/%s", t.name))
			default:
				logger.Debug(
					fmt.Sprintf("target ping wait: received msg =[%s] from collector", Msg2Text(submsg)),
					"collid", fmt.Sprintf("ping/%s", t.name))
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

	if err != nil {
		met_ch <- NewInvalidMetric(t.logContext, err)
		targetUp = false
	}
	if t.name != "" {
		// Export the target's `up` metric as early as we know what it should be.
		met_ch <- NewMetric(t.upDesc, boolToFloat64(targetUp), nil, nil)
	}
	if health_only {
		return
	}

	// Don't bother with the collectors if target is down.
	if targetUp {
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
			switch msg {

			case MsgLogin:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					fmt.Sprintf("target: received MsgLogin / check has_logged: %v", has_logged),
					"collid", t.name)
				// check if we have already logged to target ?
				t.client.symtab["__collector_id"] = t.name
				if msg == MsgLogin && !has_logged {
					//
					// wait until all collectors have finished
					wg_coll.Wait()
					t.client.Clear()
					t.client.symtab["__coll_channel"] = collectChan

					if status, err := t.client.Login(); err != nil {
						collectChan <- MsgQuit
						if err == ErrInvalidLogin || err == ErrInvalidLoginNoCipher || err == ErrInvalidLoginInvalidCipher {
							for _, c := range t.collectors {
								c.SetStatus(CollectorStatusInvalidLogin)
							}

						}
					} else {
						if status {
							has_logged = true
							collectChan <- MsgCollect
						} else {
							collectChan <- MsgQuit
						}
					}
					delete(t.client.symtab, "__coll_channel")
				}
				delete(t.client.symtab, "__collector_id")

			case MsgCollect:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"target: received MsgCollect",
					"collid", t.name)
				wg_coll.Add(len(t.collectors))
				logger.Debug(
					fmt.Sprintf("target: send %d collector(s)", len(t.collectors)),
					"collid", t.name)

				t.client.symtab["__coll_channel"] = collectChan

				for _, c := range t.collectors {
					t.client.symtab["__collector_id"] = fmt.Sprintf("%s#%s", t.name, c.GetId())
					// have to build a new client copy to allow multi connection to target
					c_client := t.client.Clone(t.config)
					c.SetClient(c_client)

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
							if r := recover(); r != nil {
								err, ok := r.(error)
								if !ok {
									err = errors.New("undefined error")
								}
								logger.Debug(
									fmt.Sprintf("collector has panic-ed: %s", err.Error()),
									"collid", t.name)
							}
						}()
						defer func() {
							cancel()
							wg_coll.Done()
						}()
						collector.Collect(coll_ctx, met_ch, collectChan)
					}(c, t.GetDeadline())
				}
				logger.Debug(
					"target: send MsgWait",
					"collid", t.name)
				delete(t.client.symtab, "__collector_id")

				collectChan <- MsgWait

			case MsgTimeout:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"target: received MsgTimeout",
					"collid", t.name)

				if !has_quitted {
					close(collectChan)
					has_quitted = true
				}

			case MsgQuit:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"target: received MsgQuit",
					"collid", t.name)

				if !has_quitted {
					close(collectChan)
					has_quitted = true
				}

			case MsgWait:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"start waiting for all collectors are over",
					"collid", t.name)

				wg_coll.Wait()
				t.content_mutex.Lock()
				logger = t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"after waiting for all collectors is over",
					"collid", t.name)
				need_login := false
				// we have received len(t.collectors) msgs from collectors
				for i := 0; i < len(t.collectors); i++ {
					logger.Debug(
						fmt.Sprintf("target wait: read msg[%d] from collector", i),
						"collid", t.name)
					submsg := <-collectChan

					switch submsg {
					case MsgLogin:
						logger.Debug(
							"target wait: received MsgLogin",
							"collid", t.name)
						need_login = true
					default:
						logger.Debug(
							fmt.Sprintf("target wait: received msg[%d]=[%s] from collector", i, Msg2Text(submsg)),
							"collid", t.name)
					}
				}
				if need_login {
					collectChan <- MsgLogin
					t.client.symtab["logged"] = false
					has_logged = false
					if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
						logger.Debug(
							fmt.Sprintf("target: MsgLogin check has_logged: %v", logged),
							"collid", t.name)
					}
				} else {
					collectChan <- MsgQuit
				}
			}
		}
		t.logger.Debug(
			"goroutine target collector controler is over",
			"collid", t.name)

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
	logger.Debug("collectors have stopped")

	if t.name != "" {
		// And exporter a `collector execution status` metric for each collector once we're done scraping.
		if targetUp {
			labels_name := make([]string, 1)
			labels_name[0] = "collectorname"
			labels_value := make([]string, 1)
			for _, c := range t.collectors {
				labels_value[0] = c.GetName()
				logger.Debug(
					fmt.Sprintf("target collector['%s'] collid=[%s] has status=%d", labels_value[0], c.GetId(), c.GetStatus()),
					"collid", t.name)

				met_ch <- NewMetric(t.collectorStatusDesc, float64(c.GetStatus()), labels_name, labels_value)

				// obtain set_stats from collector
				c.SetSetStats(t)
			}
		}
		// And exporter a `scrape duration` metric once we're done scraping.
		met_ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil, nil)
	}
}

// ping implement ping for target
func (t *target) ping(coll_ch chan<- int) (bool, error) {
	t.client.symtab["__coll_channel"] = coll_ch
	status, err := t.client.Ping()
	if err != nil {
		switch err {
		case ErrInvalidLogin:
			if t.has_ever_logged {
				t.logger.Error(
					err.Error(),
					"collid", fmt.Sprintf("ping/%s", t.name))
			}
			coll_ch <- MsgLogin
		case ErrContextDeadLineExceeded:
			coll_ch <- MsgTimeout
		default:
			coll_ch <- MsgQuit
		}
	} else {
		coll_ch <- MsgDone
	}
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
