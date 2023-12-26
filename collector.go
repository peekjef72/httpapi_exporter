package main

import (
	"context"
	"fmt"

	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dto "github.com/prometheus/client_model/go"
)

// Collector is a self-contained group of http queries and metric families to collect from results. It is
// conceptually similar to a prometheus.Collector.
type Collector interface {
	// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
	Collect(context.Context, chan<- Metric, chan<- int)
	SetClient(*Client)
	GetClient() *Client
	GetName() (id string)
	GetId() (id string)
	GetStatus() int
	SetLogger(log.Logger)
	SetSetStats(Target)
}

// collector implements Collector. It wraps a collection of queries, metrics and the database to collect them from.
type collector struct {
	config *CollectorConfig
	client *Client
	// queries    []*Query
	logContext     []interface{}
	collect_script []*YAMLScript
	// metricFamilies []*MetricFamily
	status int

	// to protect the data during exchange
	content_mutex *sync.Mutex
	logger        log.Logger
}

// NewCollector returns a new Collector with the given configuration and database. The metrics it creates will all have
// the provided const labels applied.
func NewCollector(
	logContext []interface{},
	logger log.Logger,
	cc *CollectorConfig,
	constLabels []*dto.LabelPair,
	collect_script []*YAMLScript) (Collector, error) {

	// var mfs []*MetricFamily

	logContext = append(logContext, "collector", cc.Name)
	// mfs := make([]*MetricFamily,)
	for _, scr := range collect_script {
		for _, ma := range scr.metricsActions {
			for _, act := range ma.Actions {
				if act.Type() == metric_action {
					mc := act.GetMetric()
					if mc == nil {
						return nil, fmt.Errorf("MetricAction nil received")
					}
					mf, err := NewMetricFamily(logContext, mc, constLabels, cc.customTemplate)
					if err != nil {
						return nil, err
					}
					//			ma.metricFamilies = append(ma.metricFamilies, mf)
					// mfs = append(mfs, mf)
					act.SetMetricFamily(mf)
				}
			}
			// for _, mc := range ma.GetMetrics() {
			// }
		}
	}
	// Instantiate metric families.
	// for _, mc := range cc.Metrics {
	// 	mf, err := NewMetricFamily(logContext, mc, constLabels, cc.customTemplate)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	mfs, found := queryMFs[mc.Query()]
	// 	if !found {
	// 		mfs = make([]*MetricFamily, 0, 2)
	// 	}
	// 	queryMFs[mc.Query()] = append(mfs, mf)
	// }

	// Instantiate queries.
	// queries := make([]*Query, 0, len(cc.Metrics))
	// for qc, mfs := range queryMFs {
	// 	q, err := NewQuery(logContext, logger, qc, mfs...)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	queries = append(queries, q)
	// }

	c := collector{
		config: cc,
		// queries:    queries,
		logContext: logContext,
		logger:     logger,
		// metricFamilies: mfs,
		collect_script: collect_script,
		content_mutex:  &sync.Mutex{},
	}

	if c.config.MinInterval > 0 {
		var logCtx []interface{}

		logCtx = append(logCtx, logContext...)
		logCtx = append(logCtx, "msg", fmt.Sprintf("NewCollector(): Non-zero min_interval (%s), using cached collector.", c.config.MinInterval))
		level.Debug(logger).Log(logCtx...)
		return newCachingCollector(&c), nil
	}
	return &c, nil
}

// GetClient implement GetClient for collector
// obtain pointer to client
func (c *collector) GetClient() (client *Client) {
	return c.client
}

// SetClient implement SetClient for collector
// obtain pointer to client
func (c *collector) SetClient(client *Client) {
	c.client = client
}

// GetId implement GetId for collector
// obtain collector id for log purpose
func (c *collector) GetId() string {
	return c.config.id
}

// GetName implement GetName for collector
// obtain collector name for collector_status metric
func (c *collector) GetName() string {
	return c.config.Name
}

// GetStatus implement GetStatus for collector
// obtain the status of collector scripts execution
func (c *collector) GetStatus() int {
	return c.status
}

func (c *collector) SetLogger(logger log.Logger) {
	c.content_mutex.Lock()
	c.logger = logger
	// c.client.logger = logger
	c.content_mutex.Unlock()
}

// SetSetStats implements SetSetStats for collector.
// Set vars from collector symbols table into target symtab.
// Lock target during action
func (c *collector) SetSetStats(target Target) {
	if len(c.collect_script) > 0 {
		for _, sc := range c.collect_script {
			if len(sc.setStatsActions) > 0 {
				if r_setstats, ok := c.client.symtab["set_stats"]; ok {
					if set_stats, ok := r_setstats.(map[string]any); ok {
						target.Lock()
						for key, value := range set_stats {
							target.SetSymbol(key, value)
						}
						target.Unlock()
					}
				}
			}
		}
	}
}

// type CollectContext struct {
// 	method func(*CallClientExecuteParams, map[string]any) error
// 	// ctx context.Context
// 	metric_ch      chan<- Metric
// 	metricfamilies []*MetricFamily
// 	wake_cond      *sync.Cond
// 	// logcontext []any
// }

// Collect implements Collector.
func (c *collector) Collect(ctx context.Context, metric_ch chan<- Metric, coll_ch chan<- int) {
	var (
		reset_coll_id bool = false
		status        int  = 0
	)

	// var wg sync.WaitGroup
	// wg.Add(len(c.queries))
	// for _, q := range c.queries {
	// 	go func(q *Query) {
	// 		defer wg.Done()
	// 		q.Collect(ctx, client, ch)
	// 	}(q)
	// }
	// // Only return once all queries have been processed
	// wg.Wait()
	// use a collect context object

	// cctx := &CollectContext{
	// 	method: c.client.callClientExecute,
	// 	// ctx:            ctx,
	// 	metric_ch:      ch,
	// 	metricfamilies: c.metricFamilies,
	// 	wake_cond:      wake_cond,
	// 	// logcontext:    c.logContext,
	// }

	c.client.symtab["__method"] = c.client.callClientExecute
	// c.client.symtab["__context"] = ctx
	c.client.symtab["__metric_channel"] = metric_ch
	c.client.symtab["__coll_channel"] = coll_ch
	// c.client.symtab["__metricfamilies"] = c.metricFamilies
	// c.client.symtab["__wake_cond"] = wake_cond
	// c.client.symtab["__logcontext"] = c.logContext

	// c.client.symtab["__collect_context"] = cctx
	cid := GetMapValueString(c.client.symtab, "__collector_id")
	if cid == "" {
		c.client.symtab["__collector_id"] = "--"
		reset_coll_id = true
	}

	c.status = 0
	status = 1
	for _, scr := range c.collect_script {
		level.Debug(c.logger).Log(
			"collid", CollectorId(c.client.symtab, c.logger),
			"msg", fmt.Sprintf("starting script '%s/%s'", c.config.Name, scr.name))
		if err := scr.Play(c.client.symtab, false, c.logger); err != nil {
			switch err {
			case ErrInvalidLogin:
				status = 2
				coll_ch <- MsgLogin
			case ErrContextDeadLineExceeded:
				status = 3
				coll_ch <- MsgTimeout
			default:
				level.Warn(c.logger).Log(
					"collid", CollectorId(c.client.symtab, c.logger),
					"script", ScriptName(c.client.symtab, c.logger),
					"errmsg", err)
				coll_ch <- MsgQuit
				status = 0
			}
			break
		}
	}

	// set collector execution status
	c.status = status

	// tell calling target that this collector is over.
	if status != 0 {
		coll_ch <- MsgDone
		level.Debug(c.logger).Log(
			"collid", CollectorId(c.client.symtab, c.logger),
			"script", ScriptName(c.client.symtab, c.logger),
			"msg", "MsgDone sent to channel.")
	}

	// clean up
	level.Debug(c.logger).Log(
		"collid", CollectorId(c.client.symtab, c.logger),
		"msg", fmt.Sprintf("removing metric channel for '%s'", c.config.Name))

	delete(c.client.symtab, "__metric_channel")
	delete(c.client.symtab, "__coll_channel")
	if reset_coll_id {
		delete(c.client.symtab, "__collector_id")
	}
}

// newCachingCollector returns a new Collector wrapping the provided raw Collector.
func newCachingCollector(rawColl *collector) Collector {
	cc := &cachingCollector{
		rawColl:     rawColl,
		minInterval: time.Duration(rawColl.config.MinInterval),
		cacheSem:    make(chan time.Time, 1),
	}
	cc.cacheSem <- time.Time{}
	return cc
}

// Collector with a cache for collected metrics. Only used when min_interval is non-zero.
type cachingCollector struct {
	// Underlying collector, which is being cached.
	rawColl *collector
	// Convenience copy of rawColl.config.MinInterval.
	minInterval time.Duration

	// Used as a non=blocking semaphore protecting the cache. The value in the channel is the time of the cached metrics.
	cacheSem chan time.Time
	// Metrics saved from the last Collect() call.
	cache []Metric
}

// SetClient implement SetClient()for cachingCollector
func (cc *cachingCollector) SetClient(client *Client) {
	cc.rawColl.client = client
}

// SetClient implement SetClient for cachingCollector
func (cc *cachingCollector) GetClient() (client *Client) {
	return cc.rawColl.client
}

// GetName implement GetName for cachingCollector
// obtain collector name for collector_status metric
func (cc *cachingCollector) GetName() (id string) {
	return cc.rawColl.config.Name
}

// GetId implement GetId for cachingCollector
func (cc *cachingCollector) GetId() (id string) {
	return cc.rawColl.config.id
}

// GetStatus implement GetStatus for collector
// obtain the status of collector scripts execution
func (cc *cachingCollector) GetStatus() int {
	return cc.rawColl.status
}

func (cc *cachingCollector) SetLogger(logger log.Logger) {
	cc.rawColl.SetLogger(logger)
}

func (cc *cachingCollector) SetSetStats(target Target) {
	cc.rawColl.SetSetStats(target)
}

// Collect implements Collector.
func (cc *cachingCollector) Collect(ctx context.Context, ch chan<- Metric, coll_ch chan<- int) {
	if ctx.Err() != nil {
		ch <- NewInvalidMetric(cc.rawColl.logContext, ctx.Err())
		return
	}

	collTime := time.Now()
	select {
	case cacheTime := <-cc.cacheSem:
		// Have the lock.
		if age := collTime.Sub(cacheTime); age > cc.minInterval {
			// Cache contents are older than minInterval, collect fresh metrics, cache them and pipe them through.
			var logCtx []interface{}

			logCtx = append(logCtx, cc.rawColl.logContext...)
			logCtx = append(logCtx, "msg", fmt.Sprintf("Collecting fresh metrics: min_interval=%.3fs cache_age=%.3fs",
				cc.minInterval.Seconds(), age.Seconds()))
			level.Debug(cc.rawColl.logger).Log(logCtx...)
			cacheChan := make(chan Metric, capMetricChan)
			cc.cache = make([]Metric, 0, len(cc.cache))
			go func() {
				cc.rawColl.Collect(ctx, cacheChan, coll_ch)
				close(cacheChan)
			}()
			for metric := range cacheChan {
				cc.cache = append(cc.cache, metric)
				ch <- metric
			}
			cacheTime = collTime
		} else {
			var logCtx []interface{}

			logCtx = append(logCtx, cc.rawColl.logContext...)
			logCtx = append(logCtx, "msg", fmt.Sprintf("Returning cached metrics: min_interval=%.3fs cache_age=%.3fs",
				cc.minInterval.Seconds(), age.Seconds()))
			level.Debug(cc.rawColl.logger).Log(logCtx...)
			for _, metric := range cc.cache {
				ch <- metric
			}
		}
		// Always replace the value in the semaphore channel.
		cc.cacheSem <- cacheTime

	case <-ctx.Done():
		// Context closed, record an error and return
		// TODO: increment an error counter
		ch <- NewInvalidMetric(cc.rawColl.logContext, ctx.Err())
	}
}
