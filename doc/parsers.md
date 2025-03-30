
# Parsers

Parsers are used to build structured objects with the content obtained by a query function in the collectors scripts.

The default parser in json, because historically httpapi_exporter was designed to work only with server that returns json content.

## Currently defined parsers are

- [json](#json)
- [none](#none-parser)
- [prometheus](#prometheus)
- [text-lines](#text-lines)
- [xml](#xml)
- [yaml](#yaml)

## Usage

The parser attribute applies only to the query action.

```yaml
    - name: collect elements
      query:
        url: /reports/summary/overview
        var_name: results
        parser: json

```

This above example tells to parse the result content as a json object called "results" in the symbol table, so it can be use and traverse for further operations.

## JSON

The json parser returns internally a go `map[string]any` if json returns an `object` or `[]any` if it is an `array`.

Imagine the returned content is:

```json
  {
    "sampleTime": "2022-11-10T18:25:00+01:00",
    "sampleTimeSec": 1668101100,
    "total": 24,
    "members": [
      {"node":0, "cpu":0, "userPct": 1.8, "systemPct":8.5, "idlePct":89.7, "interruptsPerSec":40452.9, "contextSwitchesPerSec":84915.9},
      {"node":0, "cpu":1, "userPct":1.2, "systemPct":24.8, "idlePct":73.9, "interruptsPerSec":0.0, "contextSwitchesPerSec":0.0},
      ...
    ]
  }
```

According to the previous config, you have a variable `result` containing this object; something that you can write in javascript:

```javacript
result = {
    "sampleTime": "2022-11-10T18:25:00+01:00",
    "sampleTimeSec": 1668101100,
    "total": 24,
    "members": [
      {"node":0, "cpu":0, "userPct": 1.8, "systemPct":8.5, "idlePct":89.7, "interruptsPerSec":40452.9, "contextSwitchesPerSec":84915.9},
      {"node":0, "cpu":1, "userPct":1.2, "systemPct":24.8, "idlePct":73.9, "interruptsPerSec":0.0, "contextSwitchesPerSec":0.0},
      ...
    ]
}
```

So `{{ .results.members | toRawJson }}` write in gotemplate syntax or `$results.members` in exporter syntax, is a list of object that you can loop on to obtain each "node".

with following config:

```yaml
    - name: collect data cpu
      scope: results
      metrics:
        - metric_name: cpu_usage_percent
          help: cpu percent usage over last 5 min for system, user and idle (labeled mode) by node and cpu core
          type: gauge
          key_labels:
            node: _
            cpu: _
          value_label: mode
          values:
            user:   $userPct
            system: $systemPct
            idle:   $idlePct
          loop: $members
```

This block of configuration is a two levels instuction set:

1) `metrics` with scope

    here `scope` directive tells to reduce the symbols table to only 'results" variable, so ease the writing of formula by compacting the syntax; anyway you can perfectly loop on $resuls.members.
1) `metric_name` with `loop` and implicit scope on loop_var. It means to loop on each $results.members; Loop on metric_name has a specific behavior that is to scope on the loop_var; here `loop_var` is not set so is defined to default `item` value; As conclusion, `$userPct` is equivalent to `{{ .item.userPct }}`.

It is quite complicated to explain, but the result is in fact very simple and natural: you want a metric called `cpu_usage_percent` labeled by each `node` and `cpu` and with the `mode` that is corresponding to cpu time spent percent into that state.

So for the first `member`:

```json
{"node":0, "cpu":0, "userPct": 1.8, "systemPct":8.5, "idlePct":89.7, "interruptsPerSec":40452.9, "contextSwitchesPerSec":84915.9}
```

- node is 0
- cpu is 0
- userPct is 1.8
- systemPct is 8.5
- idlePct is 89.7

so generated metric will be:

```text
# HELP cpu_usage_percent pu percent usage over last 5 min for system, user and idle (labeled mode) by node and cpu core
# TYPE cpu_usage_percent gauge
cpu_usage_percent{node="0",cpu="0",mode="user"} 1.8
cpu_usage_percent{node="0",cpu="0",mode="system"} 8.5
cpu_usage_percent{node="0",cpu="0",mode="idel"} 89.7
```

And so on for all node elements.

## YAML

The `yaml` parser works exactly like the json parser. I do not have usecases for the moment, so can't imagine what should be specific.

Once the content is parsed, the resulting object is constructed as a JSON object and is therefore treated as such.

## XML

The `xml` parser has the same purpose than the previous two: build a go `map[string]any` that represents the objects and their attributes so that they can be addressed simply by standard '.' notation.

by example, the bolow xml data will produce:

```xml
<DeploymentStatus xmlns="exa:exa.bee.deploy.v10">
    <HostStatus hostname="host.domain" install="cvslave" status="ok" architecture="amd64-linux" nbCpus="4" cpuUsage="39.19598"
    dataDir="/local/data/structma/datadir" installDir="/opt/CloudView/linux_a64/cv"
    user="user" hostAgentStartupConfigVersion="31" exaHostAgentPort="61009" javaHostAgentPort="61012">
        <ProcessStatus 
            processName="index6-bg0-i1" 
            status="started" pid="128401"
            lastStartDate="1650578601275"
            nbUnexpectedRestarts="0"
            loopCrashing="false"
            nbConsecutiveUnexpectedRestarts="0"
            ports="61001" debugPort="-1" defaultPort="61001"
            jmxPort="-1"/>
    ...
```

in json notation:

```json
results = {
    "DeploymentStatus": {
        "xmlns": "exa:exa.bee.deploy.v10"
        "HostStatus": [ {
            "hostname": "host.domain",
            "install": "cvslave",
            "status": "ok",
            "architecture": "amd64-linux",
            "nbCpus": "4",
            "cpuUsage": "39.19598",
            "dataDir": "/local/data/structma/datadir",
            "installDir": "/opt/CloudView/linux_a64/cv",
            "user": "user",
            "hostAgentStartupConfigVersion": "31",
            "exaHostAgentPort": "61009",
            "javaHostAgentPort": "61012",
            "ProcessStatus": [{
                "ProcessName": "index6-bg0-i1",
                "status": "started",
                "pid": "128401",
                "lastStartDate": "1650578601275",
                "nbUnexpectedRestarts": "0",
                "loopCrashing": false,
                "nbConsecutiveUnexpectedRestarts": "0",
                "ports": "61001",
                "debugPort": "-1",
                "defaultPort": "61001",
                "jmxPort": "-1"
            },
            {...}]
        },
        {...}
        ]
    }
}
```

In details, each subelement of an element is represented as a array even if the element is unique. In previous example, subelement `HostStatus` of element `DeploymentStatus` will be represented in object as a single element list. Same for `ProcessStatus` in `HostStatus`.

## None Parser

This parser is a do nothing parser: it means you don't want to use the result content so it is useless to perform any operation on it. That may be usefull for "ping" page without any interesting content.

## text-lines

This parser is intended to work with plain text content. It splits the content into lines after each carriage return (regexp pattern "\r?\n"), thus transforming the content into a array of strings. Then each line can be looped to search, extract data from ...

e.g.
You collect a generated status page with "status" text written into:

```text
STATUS: OK
an another line
```

You can use a "text-lines" parser to read the content and build a metric on the status content:

```yaml
scripts:
  check_status:
    - name: get status page
      query:
        url: /status
        var_name: results
        # debug: true
        parser: text-lines
    # $results content
    # => text/plain
    #   STATUS:OK
    - name: debug virtualbrowser_status
      # get fist line, converted to string, regexpExtract group 1
      # {{ exporterRegexExtract "^STATUS:\\s*(.+)$" (toString (index .results 0)) }}
      set_fact:
        status: >
          {{- index (exporterRegexExtract "^STATUS:\\s*(.+)$" (toString (index .results 0) ) ) 1 -}}

    - name: proceed elements
      scope: none
      metrics:
        - metric_name: access_status
          help: "status value returned by /status url: 1:OK - 0: KO"
          type: gauge
          values:
            _: '{{ if EQ .status "OK" }}1{{ else }}0{{ end }}'
```

## Prometheus

This parser is intended to work with prometheus exporter content. It will build an go map[string]any object; each key of the object is metric name, and the value the metric definition with labels and value.

e.g.:

for simple type `gauge` or `counter`:

```text
# HELP apache_workers Apache worker statuses
# TYPE apache_workers gauge
apache_workers{state="busy"} 1
apache_workers{state="idle"} 99
# HELP nrpe_up Indicates whether or not nrpe agent is ip
# TYPE nrpe_up gauge
nrpe_up 1
```

will produce an object:

```json
results = {
    "apache_workers": {
        "name": "apache_workers",
        "help": "Apache worker statuses",
        "type": "gauge",
        "metrics": [{
                "labels": {
                    "state": "busy"
                },
                "value": 1
            },{
                "labels": {
                    "state": "idle"
                },
                "value": 99
        }]
    },
    "nrpe_up": {
        "name": "nrpe_up",
        "help": "Indicates whether or not nrpe agent is ip",
        "type": "gauge",
        "metrics": [{
                "labels": {},
                "value": 1
        }]
    }
}
```

for metric type `summary` or `histogram`:

```text
# HELP go_gc_duration_seconds A summary of the pause duration of garbage collection cycles.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 3.9464e-05
go_gc_duration_seconds{quantile="0.25"} 5.5593e-05
go_gc_duration_seconds{quantile="0.5"} 7.7457e-05
go_gc_duration_seconds{quantile="0.75"} 0.00010531
go_gc_duration_seconds{quantile="1"} 0.015503494
go_gc_duration_seconds_sum 0.803447317
go_gc_duration_seconds_count 4382
# HELP go_sched_pauses_total_gc_seconds Distribution of individual GC-related stop-the-world pause latencies. This is the time from deciding to stop the world until the world is started again. Some of this time is spent getting all threads to stop (this is measured directly in /sched/pauses/stopping/gc:seconds), during which some threads may still be running. Bucket counts increase monotonically.
# TYPE go_sched_pauses_total_gc_seconds histogram
go_sched_pauses_total_gc_seconds_bucket{le="6.399999999999999e-08"} 0
go_sched_pauses_total_gc_seconds_bucket{le="6.399999999999999e-07"} 0
go_sched_pauses_total_gc_seconds_bucket{le="7.167999999999999e-06"} 11218
go_sched_pauses_total_gc_seconds_bucket{le="8.191999999999999e-05"} 25778
go_sched_pauses_total_gc_seconds_bucket{le="0.0009175039999999999"} 29390
go_sched_pauses_total_gc_seconds_bucket{le="0.010485759999999998"} 29406
go_sched_pauses_total_gc_seconds_bucket{le="0.11744051199999998"} 29406
go_sched_pauses_total_gc_seconds_bucket{le="+Inf"} 29406
go_sched_pauses_total_gc_seconds_sum 0.422120704
go_sched_pauses_total_gc_seconds_count 29406
```

will produce an object:

```json
results = {
    "go_gc_duration_seconds": {
        "name": "go_gc_duration_seconds",
        "type": "summary",
        "help": "A summary of the pause duration of garbage collection cycles.",
        "metrics": [
            {
                "labels": {},
                "summary": {
                "quantile": [
                    {
                    "quantile": 0,
                    "value": 0.000039464
                    },
                    {
                    "quantile": 0.25,
                    "value": 0.000055593
                    },
                    {
                    "quantile": 0.5,
                    "value": 0.000077457
                    },
                    {
                    "quantile": 0.75,
                    "value": 0.00010531
                    },
                    {
                    "quantile": 1,
                    "value": 0.015503494
                    }
                ],
                "sample_count": 4382,
                "sample_sum": 0.803447317
                }
            }
        ],
    },
    "go_sched_pauses_total_gc_seconds": {
        "name": "go_sched_pauses_total_gc_seconds",
        "help": "Distribution of individual GC-related stop-the-world pause latencies. This is the time from deciding to stop the world until the world is started again. Some of this time is spent getting all threads to stop (this is measured directly in /sched/pauses/stopping/gc:seconds), during which some threads may still be running. Bucket counts increase monotonically.",
        "type": "histogram",
        "metrics": [{
            "labels": {},
            "histogram": {
                "sample_count": 29406,
                "sample_sum": 0.422120704,
                "buckets": [{
                    "le": "6.399999999999999e-08",
                    "value": 0
                },{
                    "le": "6.399999999999999e-07",
                    "value": 0
                },{
                    "le": "7.167999999999999e-06",
                    "value": 11218
                },{
                    "le": "8.191999999999999e-05",
                    "value": 25778
                },{
                    "le": "0.0009175039999999999",
                    "value": 29390
                },{
                    "le": "0.010485759999999998",
                    "value": 29406
                },{
                    "le": "0.11744051199999998",
                    "value": 29406
                },{
                    "le": "+Inf",
                    "value": 29406
                }]
            }
        }]
    }
}
```
