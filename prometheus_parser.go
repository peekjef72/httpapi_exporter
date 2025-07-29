// cSpell:ignore quantiles
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	// cp "github.com/mitchellh/copystructure"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func ParsePrometheusResponse(data []byte) (any, error) {

	buf := bufio.NewReader(strings.NewReader(string(data)))
	// buf := bytes.NewReader(data)
	format := expfmt.NewFormat(expfmt.TypeTextPlain)
	decoder := expfmt.NewDecoder(buf, format)
	// var metrics []*dto.MetricFamily
	// results := make([]map[string]any, 0, 10)
	results := make(map[string]any)
	for {
		var (
			mf   dto.MetricFamily
			name string
		)
		if err := decoder.Decode(&mf); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("unmarshaling failed: %v", err)
		}
		res := make(map[string]any)
		name = *mf.Name
		res["name"] = name
		res["help"] = *mf.Help
		res["type"] = strings.ToLower(mf.Type.String())
		res_metrics := make([]map[string]any, 0, 10)
		for _, metric := range mf.Metric {
			res_metric := make(map[string]any)
			labels := make(map[string]string, len(metric.Label))
			if len(metric.Label) > 0 {
				for _, label := range metric.Label {
					labels[*label.Name] = *label.Value
				}
			}
			res_metric["labels"] = labels

			switch *mf.Type {
			case dto.MetricType_GAUGE:
				if metric.Gauge != nil {
					res_metric["value"] = metric.Gauge.GetValue()
				}
			case dto.MetricType_COUNTER:
				if metric.Counter != nil {
					res_metric["value"] = metric.Counter.GetValue()
				}
			case dto.MetricType_HISTOGRAM:
				if metric.Histogram != nil {
					histogram := make(map[string]any)
					histogram["sample_count"] = *metric.Histogram.SampleCount
					histogram["sample_sum"] = *metric.Histogram.SampleSum
					res_buckets := make([]any, 0)

					for _, bucket := range metric.Histogram.Bucket {
						res_bucket := make(map[string]any)
						res_bucket["le"] = fmt.Sprintf("%g", *bucket.UpperBound)
						res_bucket["value"] = *bucket.CumulativeCount
						res_buckets = append(res_buckets, res_bucket)
					}
					histogram["buckets"] = res_buckets
					res_metric["histogram"] = histogram
				}

			case dto.MetricType_SUMMARY:
				summary := make(map[string]any)
				summary["sample_count"] = *metric.Summary.SampleCount
				summary["sample_sum"] = *metric.Summary.SampleSum
				res_quantiles := make([]any, 0)

				for _, quantile := range metric.Summary.Quantile {
					res_quantile := make(map[string]float64)
					res_quantile["quantile"] = *quantile.Quantile
					res_quantile["value"] = *quantile.Value
					res_quantiles = append(res_quantiles, res_quantile)
				}
				summary["quantile"] = res_quantiles
				res_metric["summary"] = summary
			}
			res_metrics = append(res_metrics, res_metric)
		}
		if len(res) > 0 {
			res["metrics"] = res_metrics
			results[name] = res
		}
	}
	return results, nil
}
