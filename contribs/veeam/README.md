# veeam_exporter

## Overview

![dashboard overview](./screenshots/veeam_general_dash.png)

## Description
Prometheus exporter for Veeam Entreprise Manager

This exporter collects metrics from Veeam Enterprise Manager HTTP API.

It uses httpapi_exporter that exposes metrics to http (default port 9247) that can be then scrapped by Prometheus.

Several Veeam server can be polled by adding them to the YAML config file, by adding a host section:

**Config**: [see etc/config.yml](etc/veeam/config.yml)


## Usage

I recommand to create a unix symbolic link from httpapi_exporter to netscaler_exporter so it is easy to distinguish in processes tree (top, ps)

```shell
ln -s httpapi_exporter veeam_exporter
```

To start the exporter you just have to start with a path to config file:

```shell
veeam_exporter -c /etc/httpapi_exporter/veeam/config.yml
```

## exporter command line options

to start the exporter:

```shell
./veeam_exporter &
```

By default, it will load the file config.yml to perform action.

<details>
<summary>Detail options</summary>


```shell
usage: netscaler_exporter [<flags>]


Flags:
  -h, --[no-]help                Show context-sensitive help (also try --help-long and --help-man).
      --web.telemetry-path="/metrics"  
                                 Path under which to expose collector's internal metrics.
  -c, --config.file="config/config.yml"  
                                 Exporter configuration file.
  -n, --[no-]dry-run             Only check exporter configuration file and exit.
  -t, --target=TARGET            In dry-run mode specify the target name, else ignored.
  -a, --auth.key=AUTH.KEY        In dry-run mode specify the auth_key to use, else ignored.
  -o, --collector=COLLECTOR      Specify the collector name restriction to collect, replace the collector_names set for each target.
      --[no-]web.systemd-socket  Use systemd socket activation listeners instead of port listeners (Linux only).
      --web.listen-address=:9321 ...  
                                 Addresses on which to expose metrics and web interface. Repeatable for multiple addresses.
      --web.config.file=""       [EXPERIMENTAL] Path to configuration file that can enable TLS or authentication. See:
                                 https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md
      --log.level=info           Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt        Output format of log messages. One of: [logfmt, json]
  -V, --[no-]version             Show application version.


```

</details>

To test your configuration you can launch the exporter in dry_mode:

```shell
./veeam_exporter --log.level=debug -n -t host.domain
```

This command will try to connect to the 'host.domain' veeam server with parameters specified in config.yml, expose the collected metrics, and eventually the warning or errors, then exits.

## Prometheus config

Since several veeam servers can be set in the exporter, Prometheus addresses each server by adding a target parameter in the url. The "target" must be the same (lexically) that in exporter config file.

```yaml
  - job_name: "veeam"
    scrape_interval: 120s
    scrape_timeout: 60s
    metrics_path: /metrics

    static_configs:
      - targets: [ veeamhost.domain ]
        labels:
          environment: "PROD"
#    file_sd_configs:
#      - files: [ "/etc/prometheus/veeam_exp/*.yml" ]
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: "veeam-exporter-hostname.domain:9247"  # The veeam exporter's real hostname.

```
## Metrics

The collected metrics are defined in separeted files positionned the folder conf/metrics.
All Values, computations, labels are defined in the metrics files, meaning that the exporter does nothing internally on values. The configuration fully drives how values are rendered.

### Collected Metrics

All metrics are defined in the configuration file (conf/metrics/*.yml). You can retrive all metrics' names here. Most of them have help text too.

by default the history of backup job and tasks are set to 12h.
You can update this value in config file for the init script; the format is the golang time.duration so hours (Xh) or second (Ys) (or days (Zd))

```yml
httpapi_config:
  init:
    - name: default headers
      set_fact:
        headers:
          - name: "Content-Type"
            value: application/json
          - name: Accept
            value: "application/json"
        base_url: /api
        verifySSL: false
        # set default time in hours (time.Duration) to look back for jobs and tasks
        jobHistory: -12h
        taskHistory: -12h

```
file | domain | metrics
---- | ------ | -------
veeam_overview_metrics.yml | general results | count by type "backup", "proxy", "repository", "scheduled_jobs", "successful_vms", "warning_vms"
vm_overview_metrics.yml | general vm results | VMs count by protection type "protected","backedup","replicated","restore_points"<br>VMs total size in bytes by type "full_backup_points", "incremental_backup_points", "replica_restore_points", "source_vms"<br>percent of sucessful backup of VMs
repositories_metrics.yml | repositories | total and free size and in bytes of each repository by name and type
jobs_overview_metrics.yml | jobs generics | various count of job types "running", "scheduled", "scheduled_backup" "scheduled_replica_jobs_count"<br>total number of job runs by type "total", "successfull", "warning", "failed"<br>max duration for job by type and name of longuest
backup_agent_metrics.yml | backup agent | backup agent status 1 Online / 2 Offline labeled by nae , type and version
backup_servers_metrics.yml | backup servers | config of each backup server labeled by name, description, port, version: no value collect (1 returned)
backup_jobs_sessions_metrics.yml | backup jobs runs | last backup job run info state, duration, retries labeled by backup server, jobname, jobtype
vm_backup_jobs_sessions_metrics.yml| vm backup jobs runs | last vm backup job runs info state, duration, retries, total_bytes labeled by backup server, jobname, vmname, taskname, message

## Extending metrics

Exported metrics, are defined the YAML config file. The value can use Jinja2 templating language. The format of the configuration is inspired from Ansible task representation.
So a metric configuration file, consists in a list of action to perform.

There are five possible actions:

- url: to collect metrics from HTTP API
- set_fact: to assign vlaue to variables
- actions: to perform a list of (sub-)actions
- metrics: to define metrics to expose/return to Prometheus
- debug: to display debug text to logger.

All actions have default "attributes":

- name: the name of action or metric counter for metrics action.
- vars: to set vars to global symbols' table.
- with_items: to loop on current action with a list of items.
- loop_var: to set the name of the variable that will receive the current value in the loop. Default is 'item'.
- when: a list of condition (and) that must be check and be true to perform the action.

The "attributes" are analyzed in the order specified in previous table; it means that you can't use "item" var (obtained from 'with_items' directive) in the vars section because it is not yet defined when the 'vars' section is evaluated. If you need that feature, you will have to consider 'with_items' in an 'actions' section (see metrics/backup_jobs_sessions_metrics.yml).

action | parameter | description | remark
------ | ----------- | ------ | ------
url | &nbsp; |a string that's representing the entity to collect without '/api' | http://host.domain:port/api**[url]**. e.g.: /reports/summary/overview
 &nbsp; | var_name |the name of the variable to store the results. Default is '_root' meaning that the resulting JSON object is directly store in symbols table. | &nbsp;
 &nbsp; | &nbsp; | &nbsp; | &nbsp; 
 set_fact | &nbsp; | list of variable to define | &nbsp; 
 &nbsp; | var_name: value| &nbsp;  
 &nbsp; | &nbsp; | &nbsp; | &nbsp; 
metrics | &nbsp; | define the list of metrics to expose
 &nbsp; | metric_prefix | a prefix to add to all metric name | final name will be [metric_prefix]_[metric_name]
 'a metric' | name | the name of the metric
 &nbsp; | help | the help message added to the metric (and displayed in grafana explorer)
 &nbsp; | type 'gauge' or 'counter' | the type of the prometheus metric | &nbsp;
 &nbsp; | value | the numeric value itself | &nbsp;
 &nbsp; | labels | a list of name value pairs to qualify the metric | &nbsp;

