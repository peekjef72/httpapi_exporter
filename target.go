// cSpell:ignore collectorname, Histo, qmgr, vartype, colls

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/imdario/mergo"
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
	queryStatusName     = "query_status"
	queryStatusHelp     = "query http status label by phase(url): http return code"
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
	GetSpecificCollector() []Collector
	SetSpecificCollectorConfig(coll map[string]*CollectorConfig) error
	SetTimeout(time.Duration)
}

// target implements Target. It wraps a httpAPI, which is initially nil but never changes once instantiated.
type target struct {
	name       string
	config     *TargetConfig
	client     *Client
	collectors []Collector
	// httpAPIScript       map[string]*YAMLScript
	upDesc              MetricDesc
	scrapeDurationDesc  MetricDesc
	collectorStatusDesc MetricDesc
	queryStatusDesc     MetricDesc

	logContext []any

	logger   *slog.Logger
	deadline time.Time

	has_ever_logged bool

	// user specific collector reference
	collector_config    map[string]*CollectorConfig
	specific_collectors []Collector

	// to store query_status results
	queries_status map[string]any

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

func build_ConstantLabels(labels map[string]string) []*dto.LabelPair {
	constLabelPairs := make([]*dto.LabelPair, 0, len(labels))
	for n, v := range labels {
		constLabelPairs = append(constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(constLabelPairs))
	return constLabelPairs
}

// NewTarget returns a new Target with the given instance name, data source name, collectors and constant labels.
// An empty target name means the exporter is running in single target mode: no synthetic metrics will be exported.
func NewTarget(
	logContext []interface{},
	tPar *TargetConfig,
	gc *GlobalConfig,
	profile *Profile,
	logger *slog.Logger) (Target, error) {

	if tPar.Name != "" {
		logContext = append(logContext, "target", tPar.Name)
	}

	constLabelPairs := build_ConstantLabels(tPar.Labels)

	queryStatusDesc := NewAutomaticMetricDesc(logContext,
		profile.MetricPrefix+"_"+queryStatusName,
		gc.QueryStatusHelp,
		dto.MetricType_GAUGE, constLabelPairs,
		"phase")

	collectors := make([]Collector, 0, len(tPar.collectors))
	for _, cc := range tPar.collectors {
		csCrl := make([]*YAMLScript, len(cc.CollectScripts))
		i := 0
		for _, cs := range cc.CollectScripts {
			csCrl[i] = cs
			i++
		}
		c, err := NewCollector(logContext, logger, cc, constLabelPairs, csCrl)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, c)
	}

	upDesc := NewAutomaticMetricDesc(logContext,
		profile.MetricPrefix+"_"+upMetricName,
		gc.UpMetricHelp,
		dto.MetricType_GAUGE, constLabelPairs)

	scrapeDurationDesc := NewAutomaticMetricDesc(logContext,
		profile.MetricPrefix+"_"+scrapeDurationName,
		gc.ScrapeDurationHelp,
		dto.MetricType_GAUGE, constLabelPairs)

	collectorStatusDesc := NewAutomaticMetricDesc(logContext,
		profile.MetricPrefix+"_"+collectorStatusName,
		gc.CollectorStatusHelp,
		dto.MetricType_GAUGE, constLabelPairs,
		"collectorname")

	// testHisto := prometheus.NewHistogramVec(
	// 	prometheus.HistogramOpts{
	// 		Namespace: profile.MetricPrefix,
	// 		Name:      "qmgr_messages_inserted_size_bytes",
	// 		Help:      "Size of messages inserted into the mail queues in bytes.",
	// 		Buckets:   []float64{1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9},
	// 	},
	// 	[]string{"bla"},
	// )

	t := target{
		name:       tPar.Name,
		config:     tPar,
		client:     newClient(tPar, profile.Scripts, logger, gc),
		collectors: collectors,
		// httpAPIScript:       profile.Scripts,
		upDesc:              upDesc,
		scrapeDurationDesc:  scrapeDurationDesc,
		collectorStatusDesc: collectorStatusDesc,
		queryStatusDesc:     queryStatusDesc,
		logContext:          logContext,
		logger:              logger,
		content_mutex:       &sync.Mutex{},
		queries_status:      make(map[string]any),
	}
	if t.client == nil {
		return nil, errors.New("internal http client undefined")
	}
	// shared content mutex between target and client
	t.client.content_mutex = t.content_mutex

	t.client.SetScriptName("init")
	t.client.symtab["__collector_id"] = t.name
	// populate symtab for all query actions with [query_]status set 0 http_code (uninitialized)
	for _, c := range t.collectors {
		c.SetQueriesStatus(t.client, t.queries_status, 0)
	}

	for _, scr := range t.config.profile.Scripts {
		// some default script may be defined to null (login, clear, logout...)
		if scr == nil {
			continue
		}
		// populate MetricFamily with context for all metrics actions from profile default scripts
		for _, ma := range scr.metricsActions {
			for _, act := range ma.Actions {
				if act.Type() == metric_action {
					mc := act.GetMetric()
					if mc == nil {
						return nil, errors.New("MetricAction nil received")
					}
					mf, err := NewMetricFamily(logContext, mc, constLabelPairs, nil)
					if err != nil {
						return nil, err
					}
					//			ma.metricFamilies = append(ma.metricFamilies, mf)
					// mfs = append(mfs, mf)
					act.SetMetricFamily(mf)
				}
			}
		}
		// populate symtab for all query actions with [query_]status set to 0
		for _, act := range scr.queryActions {
			if act.Type() == query_action {
				if bool(act.Query.Status) {
					if act.Query.query.vartype == field_raw {
						url := act.Query.query.raw
						t.queries_status[url] = 0
					}
				}
			}
		}
	}
	t.client.SetScriptName("__delete__")
	delete(t.client.symtab, "__collector_id")

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

// Setter for timeout
func (t *target) SetTimeout(timeout time.Duration) {
	t.client.client.SetTimeout(timeout)
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

func (t *target) GetSpecificCollector() []Collector {
	if t.collector_config != nil {
		return t.specific_collectors
	}
	return nil
}

func (t *target) buildCollector(logger *slog.Logger,
	coll_config *CollectorConfig,
	constLabelPairs []*dto.LabelPair,
) (coll Collector, err error) {
	if constLabelPairs == nil {
		constLabelPairs = build_ConstantLabels(t.config.Labels)
	}
	csCrl := make([]*YAMLScript, len(coll_config.CollectScripts))
	i := 0
	for _, cs := range coll_config.CollectScripts {
		csCrl[i] = cs
		i++
	}
	coll, err = NewCollector(t.logContext, logger, coll_config, constLabelPairs, csCrl)

	return
}
func (t *target) SetSpecificCollectorConfig(colls map[string]*CollectorConfig) error {
	t.collector_config = colls

	if len(colls) > 0 {
		coll_list := make([]Collector, 0, len(colls))
		var (
			constLabelPairs []*dto.LabelPair
			err             error
		)
		t.content_mutex.Lock()
		logger := t.logger
		t.content_mutex.Unlock()

		for coll_name, coll_config := range colls {
			// there was no previous specific collectors... build list
			if t.specific_collectors == nil {
				coll, err := t.buildCollector(logger, coll_config, constLabelPairs)
				if err != nil {
					return err
				}
				coll_list = append(coll_list, coll)
				// else has previous, need to check if name found in list
			} else {
				for _, coll := range t.specific_collectors {
					if coll.GetName() != coll_name {
						coll, err = t.buildCollector(logger, coll_config, constLabelPairs)
						if err != nil {
							return err
						}
					}
					coll_list = append(coll_list, coll)
				}
			}
		}
		if len(coll_list) > 0 {
			t.specific_collectors = coll_list
		} else {
			t.specific_collectors = nil
		}
	}
	return nil

}

func (t *target) SetQueriesStatus(code int) {
	queries_status := t.queries_status
	if queries_status == nil {
		queries_status = make(map[string]any)
	}
	for url, raw_status := range queries_status {
		if _, ok := raw_status.(int); ok {
			queries_status[url] = code
		}
	}
	t.queries_status = queries_status

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
		colls       []Collector
		colls_init  bool = false
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
	t.client.symtab["__metric_channel"] = met_ch
	t.client.symtab["__coll_channel"] = collectChan
	msg_done_count := 0
	t.client.SetContext(ctx)

	// determine list of collectors: required in any cases to send status
	if !health_only {
		if colls = t.GetSpecificCollector(); colls != nil {
			t.SetSpecificCollectorConfig(nil)
		} else {
			colls = t.collectors
		}
		t.SetQueriesStatus(0)
	}

	for msg := range collectChan {
		switch msg {

		case MsgLogin:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"target: received MsgLogin",
				"coll", fmt.Sprintf("ping/%s", t.name))
			if msg == MsgLogin && !has_logged {
				t.client.Clear()
				if status, err := t.client.Login(); err != nil {
					t.logger.Error(
						err.Error(),
						"coll", fmt.Sprintf("ping/%s", t.name),
						"script", "ping/login")
					c_status := CollectorStatusError
					c_query_status := http.StatusInternalServerError
					switch err {
					case ErrInvalidLogin, ErrInvalidLoginNoCipher, ErrInvalidLoginInvalidCipher:
						c_status = CollectorStatusInvalidLogin
						c_query_status = http.StatusForbidden
					case ErrContextDeadLineExceeded:
						c_status = CollectorStatusTimeout
						c_query_status = http.StatusGatewayTimeout
					}

					t.client.SetScriptName("login")
					for _, c := range colls {
						c.SetStatus(c_status)
						c.SetQueriesStatus(t.client, t.queries_status, c_query_status)
					}
					t.client.SetScriptName("__delete__")

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
				"coll", fmt.Sprintf("ping/%s", t.name))
			// If using a single target connection, collectors will likely run sequentially anyway. But we might have more.
			wg_coll.Add(1)
			go func(t *target, met_ch chan<- Metric, coll_ch chan<- int) {
				defer wg_coll.Done()
				targetUp, err = t.ping(collectChan)
			}(t, met_ch, collectChan)
			logger.Debug(
				"target: ping send MsgWait",
				"coll", fmt.Sprintf("ping/%s", t.name))
			collectChan <- MsgWait

		case MsgQuit:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"target: ping received MsgQuit",
				"coll", fmt.Sprintf("ping/%s", t.name))
			leave_loop = true

		// case not possible because timeout occurs during wait not in "standalone"
		// case MsgTimeout:
		// 	t.content_mutex.Lock()
		// 	logger := t.logger
		// 	t.content_mutex.Unlock()
		// 	logger.Debug(
		// 		"target: ping received MsgTimeout",
		// 		"coll", t.name)
		// 	leave_loop = true

		case MsgWait:
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"start waiting for ping is over",
				"coll", fmt.Sprintf("ping/%s", t.name))
			wg_coll.Wait()
			t.content_mutex.Lock()
			logger = t.logger
			t.content_mutex.Unlock()
			logger.Debug(
				"after waiting for ping is over",
				"coll", fmt.Sprintf("ping/%s", t.name))
			need_login := false
			// repopulate collect channel with already received message.
			for range msg_done_count {
				collectChan <- MsgDone
			}
			// read one message
			subMsg := <-collectChan

			switch subMsg {
			case MsgLogin:
				logger.Debug(
					"target ping wait: received MsgLogin",
					"coll", fmt.Sprintf("ping/%s", t.name))
				need_login = true
			case MsgDone:
				logger.Debug(
					"target ping wait: received MsgDone",
					"coll", fmt.Sprintf("ping/%s", t.name))
			case MsgTimeout:
				logger.Debug(
					"target ping wait: received MsgTimeout",
					"coll", fmt.Sprintf("ping/%s", t.name))
				t.client.SetScriptName("ping")
				for _, c := range colls {
					c.SetStatus(CollectorStatusTimeout)
					c.SetQueriesStatus(t.client, t.queries_status, http.StatusGatewayTimeout)
				}
				t.client.SetScriptName("__delete__")

			default:
				logger.Debug(
					fmt.Sprintf("target ping wait: received msg =[%s] from collector", Msg2Text(subMsg)),
					"coll", fmt.Sprintf("ping/%s", t.name))
				if err != nil {
					// status_code, _ := GetMapValueInt(t.client.symtab, "status_code")
					t.client.SetScriptName("ping")
					for _, c := range colls {
						c.SetStatus(CollectorStatusError)
						// c.SetQueriesStatus(t.client, status_code)
					}
					t.client.SetScriptName("__delete__")
				}
			}
			if need_login {
				collectChan <- MsgLogin
			} else {
				collectChan <- MsgQuit
			}
		case MsgDone:
			// we received MsgDone when we are not waiting them: it is to early!
			// store them to resend then in MsgWait sub loop
			msg_done_count += 1
		}
		// leave collectChan loop
		if leave_loop {
			break
		}
	}
	delete(t.client.symtab, "__collector_id")
	delete(t.client.symtab, "__metric_channel")
	delete(t.client.symtab, "__coll_channel")

	// if err != nil {
	// 	met_ch <- NewInvalidMetric(t.logContext, err)
	// 	targetUp = false
	// }
	if t.name != "" {
		t.content_mutex.Lock()
		logger := t.logger
		t.content_mutex.Unlock()
		logger.Debug(
			"target: send metric up result",
			"coll", t.name)
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
		msg_done_count = 0

		for msg := range collectChan {
			switch msg {

			case MsgLogin:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					fmt.Sprintf("target: received MsgLogin / check has_logged: %v", has_logged),
					"coll", t.name)
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
							for _, c := range colls {
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
					"coll", t.name)

				wg_coll.Add(len(colls))

				logger.Debug(
					fmt.Sprintf("target: send %d collector(s)", len(colls)),
					"coll", t.name)

				t.client.symtab["__coll_channel"] = collectChan

				for _, c := range colls {
					t.client.symtab["__collector_id"] = c.GetName()
					// have to build a new client copy to allow multi connection to target
					c_client := t.client.Clone(t.config)
					c_client.SetContext(ctx)
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
									"coll", t.name)
							}
						}()
						defer func() {
							// for all query actions of collector set http status_code to 504 Gateway Timeout
							// if queries_status := GetMapValueMap(c.GetClient().symtab, "__queries_status"); status != nil {
							// 	for url, raw_status := range queries_status {
							// 		if _, ok := raw_status.(int); ok {
							// 			queries_status[url] = http.StatusGatewayTimeout
							// 		}
							// 	}
							// }
							collector.SetQueriesStatus(collector.GetClient(), t.queries_status, http.StatusGatewayTimeout)
							cancel()
							wg_coll.Done()
						}()
						collector.Collect(coll_ctx, met_ch, collectChan)
					}(c, t.GetDeadline())
				}
				colls_init = true
				logger.Debug(
					"target: send MsgWait",
					"coll", t.name)
				delete(t.client.symtab, "__collector_id")

				collectChan <- MsgWait

			case MsgTimeout:
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"target: received MsgTimeout",
					"coll", t.name)

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
					"coll", t.name)

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
					"coll", t.name)

				wg_coll.Wait()
				t.content_mutex.Lock()
				logger = t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"after waiting for all collectors is over",
					"coll", t.name)
				need_login := false

				// repopulate collect channel with already received messages.
				for range msg_done_count {
					collectChan <- MsgDone
				}
				// we have received len(t.collectors) msgs from collectors
				for i := range colls {
					logger.Debug(
						fmt.Sprintf("target wait: reading msg channel for collector '%s'", colls[i].GetName()),
						"coll", t.name)
					if len(collectChan) == 0 {
						logger.Warn(
							fmt.Sprintf("target wait: msg channel for collector '%s' is empty. STOPPING", colls[i].GetName()),
							"coll", t.name)
						collectChan <- MsgQuit
						break
					}
					// read one message
					subMsg := <-collectChan

					switch subMsg {
					case MsgLogin:
						logger.Debug(
							"target wait: received MsgLogin",
							"coll", t.name)
						need_login = true
					default:
						logger.Debug(
							fmt.Sprintf("target wait: received msg for collector '%s': '%s'", colls[i].GetName(), Msg2Text(subMsg)),
							"coll", t.name)
					}
				}
				if need_login {
					collectChan <- MsgLogin
					t.client.symtab["logged"] = false
					has_logged = false
					if logged, ok := GetMapValueBool(t.client.symtab, "logged"); ok && logged {
						logger.Debug(
							fmt.Sprintf("target: MsgLogin check has_logged: %v", logged),
							"coll", t.name)
					}
				} else {
					collectChan <- MsgQuit
				}
			case MsgDone:
				// we received MsgDone when we are not waiting them: it is to early!
				// store them to resend then in MsgWait sub loop
				t.content_mutex.Lock()
				logger := t.logger
				t.content_mutex.Unlock()
				logger.Debug(
					"target: MsgDone received too early",
					"coll", t.name)
				msg_done_count += 1

			}
		}
		t.logger.Debug(
			"goroutine target collector controller is over",
			"coll", t.name)

		// play logout script if one is provided for target !
		if script, ok := t.client.sc["logout"]; ok && script != nil {
			if err := t.client.Logout(); err != nil {
				t.content_mutex.Lock()
				t.logger.Error(
					err.Error(),
					"coll", fmt.Sprintf("logout/%s", t.name))
				t.content_mutex.Unlock()

			}
		}
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
	t.client.SetContext(context.TODO())

	if t.name != "" {
		// Add to exporter a `collector execution status` metric for each collector once we're done scraping.
		if targetUp {
			labels_name := make([]string, 1)
			labels_name[0] = "collectorname"
			labels_value := make([]string, 1)
			for _, c := range colls {
				labels_value[0] = c.GetName()
				logger.Debug(
					fmt.Sprintf("target collector['%s'] coll=[%s] has status=%d", labels_value[0], c.GetName(), c.GetStatus()),
					"coll", t.name)

				met_ch <- NewMetric(t.collectorStatusDesc, float64(c.GetStatus()), labels_name, labels_value)

				// obtain set_stats from collector
				c.SetSetStats(t)
			}

			// play finalize script if one is provided for target !
			if script, ok := t.client.sc["finalize"]; ok && script != nil {
				t.client.symtab["__metric_channel"] = met_ch
				if err := t.client.Finalize(); err != nil {
					t.content_mutex.Lock()
					t.logger.Error(
						err.Error(),
						"coll", fmt.Sprintf("finalize/%s", t.name))
					t.content_mutex.Unlock()

				}
				delete(t.client.symtab, "__metric_channel")
			}
		}
		// Add to exporter a `scrape duration` metric once we're done scraping.
		met_ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil, nil)
	}

	// Add to exporter the `query status http code` metric once we've done scraping.
	// metrics may have duplicate because some query are made by target and results
	// may be "cloned" into collector symtab.
	q_status := make(map[string]int)
	for _, c := range colls {
		if colls_init {
			client := c.GetClient()
			if client != nil {
				if status := GetMapValueMap(client.symtab, "__queries_status"); status != nil {
					for url, raw_status := range status {
						status_code := raw_status.(int)
						if old_status_code, found := q_status[url]; !found || old_status_code == 0 {
							q_status[url] = status_code
						}
					}
				}
				c.SetClient(nil)
			}
		} else {
			// when target is not up! collectors haven't be initialized: use target status
			for url, raw_status := range t.queries_status {
				if _, found := q_status[url]; !found {
					q_status[url] = raw_status.(int)
				}
			}
		}
	}
	if len(q_status) > 0 {
		labels_name := make([]string, 1)
		labels_name[0] = "phase"
		labels_value := make([]string, 1)
		for url, status_code := range q_status {
			labels_value[0] = url
			t.logger.Debug(
				fmt.Sprintf("query status['%s'] has status=%d", labels_value[0], status_code),
				"coll", t.name)

			met_ch <- NewMetric(t.queryStatusDesc, float64(status_code), labels_name, labels_value)
		}
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
					"coll", fmt.Sprintf("ping/%s", t.name))
			}
			coll_ch <- MsgLogin
		case ErrContextDeadLineExceeded:
			coll_ch <- MsgTimeout
		default:
			t.logger.Error(
				err.Error(),
				"coll", fmt.Sprintf("ping/%s", t.name))
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
