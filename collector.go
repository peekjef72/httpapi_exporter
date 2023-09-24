package main

import (
	"context"
	"fmt"

	// "sync"
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
}

// collector implements Collector. It wraps a collection of queries, metrics and the database to collect them from.
type collector struct {
	config *CollectorConfig
	client *Client
	// queries    []*Query
	logContext     []interface{}
	collect_script []*YAMLScript
	// metricFamilies []*MetricFamily
	logger log.Logger
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

// SetClient implement SetClient for Client
func (c *collector) GetClient() (client *Client) {
	return c.client
}

// SetClient implement SetClient for Client
func (c *collector) SetClient(client *Client) {
	c.client = client
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
func (c *collector) Collect(ctx context.Context, ch chan<- Metric, coll_ch chan<- int) {
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
	c.client.symtab["__channel"] = ch
	c.client.symtab["__coll_channel"] = coll_ch
	// c.client.symtab["__metricfamilies"] = c.metricFamilies
	// c.client.symtab["__wake_cond"] = wake_cond
	// c.client.symtab["__logcontext"] = c.logContext

	// c.client.symtab["__collect_context"] = cctx

	for _, scr := range c.collect_script {
		level.Debug(c.logger).Log("msg", fmt.Sprintf("starting script '%s'", scr.name))
		if err := scr.Play(c.client.symtab, false, c.logger); err != nil {
			if err != ErrInvalidLogin {
				level.Warn(c.logger).Log("script", scr.name, "errmsg", err)
				coll_ch <- MsgQuit
			}
		}
	}
	delete(c.client.symtab, "__channel")
	delete(c.client.symtab, "__coll_channel")
	// delete(c.client.symtab, "__metricfamilies")
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

// SetClient implement SetClient()for Client
func (cc *cachingCollector) SetClient(client *Client) {
	cc.rawColl.client = client
}

// SetClient implement SetClient for Client
func (cc *cachingCollector) GetClient() (client *Client) {
	return cc.rawColl.client
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
