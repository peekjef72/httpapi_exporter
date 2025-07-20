# Histogram

there are two kinds of histogram that can be used with httpapi_exporter :

- external histograms that you have collected from an exporter response.
- static histograms that you can build and maintain with values collected.

## External histogram

If you retrieve data from a prometheus exporter source, then you can extract and eventually relabel or reformat it then generate a new metric.

By example, a histogram collected from a web service build with dotnet framework, that provide information through a /metric endpoint (extract):

```metrics
# HELP http_request_duration_seconds The duration of HTTP requests processed by an ASP.NET Core application.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_sum{code="200",method="GET",controller="",action="",endpoint="/incidents"} 15.055002600000014
http_request_duration_seconds_count{code="200",method="GET",controller="",action="",endpoint="/incidents"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.001"} 0
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.002"} 0
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.004"} 1400
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.008"} 2418
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.016"} 2431
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.032"} 2435
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.064"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.128"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.256"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="0.512"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="1.024"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="2.048"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="4.096"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="8.192"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="16.384"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="32.768"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",controller="",action="",endpoint="/incidents",le="+Inf"} 2443

```

You can collect the values and extract the elements you have interest for:

e.g:

```yaml
scripts:
  get dotnet_metrics:
    - name: collect elements
      query:
        url: /metrics
        var_name: results
        # debug: yes
        parser: prometheus
        trace: true

    - name: analyze results
      scope: $results
      metrics:
        - metric_name: http_request_duration_seconds
          type: histogram
          help: $http_request_duration_seconds.help
          key_labels: >-
            js: 
              delete item.labels.action
              delete item.labels.controller
              item.labels
          histogram: $item.histogram
          loop: $http_request_duration_seconds.metrics
          scope: none
```

We will obtain:

```metrics
# HELP http_request_duration_seconds The duration of HTTP requests processed by an ASP.NET Core application.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_sum{code="200",method="GET",endpoint="/incidents"} 15.055002600000014
http_request_duration_seconds_count{code="200",method="GET",endpoint="/incidents"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.001"} 0
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.002"} 0
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.004"} 1400
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.008"} 2418
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.016"} 2431
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.032"} 2435
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.064"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.128"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.256"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="0.512"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="1.024"} 2438
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="2.048"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="4.096"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="8.192"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="16.384"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="32.768"} 2443
http_request_duration_seconds_bucket{code="200",method="GET",endpoint="/incidents",le="+Inf"} 2443

```

## Statical histogram

Static histograms are persistent histograms maintained by the export across all scraping executions.
It means that you can build a local histogram of a collected value by defining :

- its name, type, help like for others metrics
- labels
- buckets definitions: they

and then after you feed the histogram with the collect value. As a result, you will obtain the repartition of that value.

By example you can build the histogram of the total response time for a specific query  with this code:

```yaml
scripts:
  get dotnet_metrics:
    - name: collect elements
      query:
        url: /metrics
        var_name: results
        # debug: yes
        parser: prometheus
        trace: true
    - metric_name: query_total_seconds
      help: total response time repartition for query
      type: histogram
      key_labels:
        page: /metrics
      histogram:
        buckets:
          - 0.001
          - 0.002
          - 0.004
          - 0.008
          - 0.016
          - 0.036
          - 0.064
          - 0.128
          - 0.256
          - 0.512
          - 1.024
          - 2.048
        value: $trace_infos.total_time

```
