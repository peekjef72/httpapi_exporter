package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"sync"
	"time"

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
	GetStatus() int
	SetStatus(status int)
	SetLogger(*slog.Logger)
	SetSetStats(Target)
	SetQueriesStatus(client *Client, queries_status map[string]any, status int)
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
	logger        *slog.Logger
}

const (
	CollectorStatusError int = iota
	CollectorStatusOk
	CollectorStatusInvalidLogin
	CollectorStatusTimeout
)

// NewCollector returns a new Collector with the given configuration and database. The metrics it creates will all have
// the provided const labels applied.
func NewCollector(
	logContext []interface{},
	logger *slog.Logger,
	cc *CollectorConfig,
	constLabels []*dto.LabelPair,
	collect_script []*YAMLScript,
) (Collector, error) {

	// var mfs []*MetricFamily

	logContext = append(logContext, "collector", cc.Name)
	// mfs := make([]*MetricFamily,)
	for _, scr := range collect_script {
		// populate MetricFamily with context for all metrics actions
		for _, ma := range scr.metricsActions {
			for _, act := range ma.Actions {
				if act.Type() == metric_action {
					mc := act.GetMetric()
					if mc == nil {
						return nil, errors.New("MetricAction nil received")
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
		}
	}

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
		logger.Debug("multilevel...", logCtx...)
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
	if c.client != nil {
		c.SetQueriesStatus(c.client, c.client.symtab, 0)
	}
}

// func (c *collector) SetQueriesStatus(client *Client, status int) {
func (c *collector) SetQueriesStatus(client *Client, queries_status map[string]any, status int) {
	// populate symtab for all query actions with [query_]status set to 0
	for _, sc := range c.collect_script {
		for _, act := range sc.queryActions {
			if act.Type() == query_action {
				if bool(act.Query.Status) {
					if act.Query.query.vartype == field_raw {
						url := act.Query.query.raw
						client.SetQueriesStatus(url, status, queries_status)
					}
				}
			}
		}
	}
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

// SetStatus implement SetStatus for collector
// set the status error of collector scripts execution
func (c *collector) SetStatus(status int) {
	c.status = status
}

func (c *collector) SetLogger(logger *slog.Logger) {
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

// Collect implements Collector.
func (c *collector) Collect(ctx context.Context, metric_ch chan<- Metric, coll_ch chan<- int) {
	var (
		reset_coll_id bool = false
		status        int  = CollectorStatusError
	)

	c.client.symtab["__method"] = c.client.callClientExecute
	c.client.symtab["__metric_channel"] = metric_ch
	c.client.symtab["__coll_channel"] = coll_ch

	cid := GetMapValueString(c.client.symtab, "__collector_id")
	if cid == "" {
		c.client.symtab["__collector_id"] = "--"
		reset_coll_id = true
	}

	c.status = CollectorStatusError
	status = CollectorStatusOk
	for _, scr := range c.collect_script {
		c.logger.Debug(
			fmt.Sprintf("starting script '%s/%s'", c.config.Name, scr.name),
			"coll", CollectorId(c.client.symtab, c.logger))
		if err := scr.Play(c.client.symtab, false, c.logger); err != nil {
			switch err {
			case ErrInvalidLogin:
				status = CollectorStatusInvalidLogin
				coll_ch <- MsgLogin
			case ErrContextDeadLineExceeded:
				status = CollectorStatusTimeout
				coll_ch <- MsgTimeout
			default:
				c.logger.Warn(
					err.Error(),
					"coll", CollectorId(c.client.symtab, c.logger),
					"script", ScriptName(c.client.symtab, c.logger))
				coll_ch <- MsgQuit
				status = CollectorStatusError
			}
			break
		}
	}

	// set collector execution status
	c.status = status

	// tell calling target that this collector is over.
	if status != CollectorStatusError {
		coll_ch <- MsgDone
		c.logger.Debug(
			"MsgDone sent to channel.",
			"coll_channel_length", fmt.Sprintf("%d", len(coll_ch)),
			"coll", CollectorId(c.client.symtab, c.logger),
			"script", ScriptName(c.client.symtab, c.logger))
	}

	// clean up
	c.logger.Debug(
		fmt.Sprintf("removing from symtab metric,coll channels vars for '%s'", c.config.Name),
		"coll", CollectorId(c.client.symtab, c.logger))

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
	cc.rawColl.SetClient(client)
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

// GetStatus implement GetStatus for cachingCollector
// obtain the status of collector scripts execution
func (cc *cachingCollector) GetStatus() int {
	return cc.rawColl.status
}

// SetStatus implement SetStatus for cachingCollector
// Set the status of collector scripts execution
func (cc *cachingCollector) SetStatus(status int) {
	cc.rawColl.status = status
}

func (cc *cachingCollector) SetLogger(logger *slog.Logger) {
	cc.rawColl.SetLogger(logger)
}

func (cc *cachingCollector) SetSetStats(target Target) {
	cc.rawColl.SetSetStats(target)
}

func (cc *cachingCollector) SetQueriesStatus(client *Client, queries_status map[string]any, status int) {
	cc.rawColl.SetQueriesStatus(client, queries_status, status)
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
			cc.rawColl.logger.Debug("multilevel", logCtx...)
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
			cc.rawColl.logger.Debug("multilevel", logCtx...)
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
