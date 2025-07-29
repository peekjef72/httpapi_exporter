<!-- cSpell:ignore healthz, SIGUSR, varname, Authconfig, symtab, collid, gotemplate, virtualbrowser, gofunc, gotest, httpapi, resty, fileglob, openmetrics, apiprefix, contribs, veeam -->
# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
## 0.4.2 / 2025-07-20

### 2025-07-27 - not release

- fixed spellings (use Code Spelling extension).
- convert and fixe up netscaler & veeam configurations to js (from go template).

### 2025-07-20 - not release

- fixed spellings (use Code Spelling extension).
- added: documentations for histograms.
- fixed: bug in preserve/restore symtab in loops that cause misbehavior with scope in metric : loop_var was sometime still defined and causing a bad default scope to undefined var.
- new: add metric type histogram. There are two usages: first to send back a parsed histogram metric from a prometheus page, second to build a locally defined metric using collected values.

### 2025-07-02 - not release

- fixed: config parsing - check for stand-alone metric_action not in metrics loop: exit with error message.
- fixed: panic in external identifiers lookup in js code.
- upgrade: to go 1.24.4, modules...

## 0.4.1 / 2025-06-04

- added: new contrib [apache_exporter configuration](./contribs/apache/README.md). a good example to introduce javascript code to build metrics.

### 2025-05-25 - not release

- added: support for env vars in target_config [see issue #1](https://github.com/peekjef72/httpapi_exporter/issues/1) :

e.g.:

```yaml
targets:
  - name: my_target
    scheme: https
    host: $env:MY_HOST_ENV_VAR_NAME
    port: 443
```

### 2025-05-16 - not release

- changed: internal "scope" directive: add error and debug msg.
- added: new parameter 'collector' to '/metrics' entry point. Collect only the specified collector(s). May be use to collect specific metrics in a particular prometheus job.

### 2025-05-07 - not release

#### BREAKING CHANGE

- renamed: variable `results_code` to `status_code`
  => update all config profiles !
- changed: in log line renamed 'collid' to 'coll' and use collector name instead of a sequential number.

### 2025-05-06 - not release

- added: support for `javascript code` in each field, as an alternative to gotemplate. This part is still in development (see examples)

- added: new metric named "query_status" (prefixed by metric_prefix) and labeled by url query stage. The goal is to provide a always generated value for each queried url with the http status code as value. The generation is conditioned by the status attribute from each query action and the default value is false.

```yaml
  - name: check api url
    query:
      url: /api/health
      method: GET
      status: true
      trace: true
```

will provide:

```text
# HELP xxx_query_status query http status label by phase(url): http return code
# TYPE xxx_query_status gauge
xxx_query_status{phase="/api/health"} 200
# HELP xxx_query_perf_seconds query stage duration in seconds
# TYPE xxx_query_perf_seconds gauge
xxx_query_perf_seconds{page="/status",stage="conn_time"} 0.004550086
xxx_query_perf_seconds{page="/status",stage="dns_lookup"} 4.3538e-05
xxx_query_perf_seconds{page="/status",stage="response_time"} 4.5354e-05
xxx_query_perf_seconds{page="/status",stage="server_time"} 0.000361965
xxx_query_perf_seconds{page="/status",stage="tcp_con_time"} 0.000111871
xxx_query_perf_seconds{page="/status",stage="tls_handshake"} 0.004329272
xxx_query_perf_seconds{page="/status",stage="total_time"} 0.004912388

```

If the request timeouts, the status code is 504.
If the request is not performed (target is down), the status code is 0.

### 2025-03-26 - not release

- added: global config parameter tls_version that allows to add old tls ciphers, because of golang change since 1.22: see [config](doc/config.md); update the netscaler default config file to use tls_version: all
- code refactored to use [cast](http://github.com/spf13/cast) for type conversion in internal functions.
- added: scripting language evolution to allow named var into expr : `$var_name.${another_varname[.attr1]}[.attr2]`
- changed: loglevel trace from warn to debug for metrics not found:

e.g.:

  ```yaml
    - metric_name: system_nodes_total
      help: total nodes in system
      type: gauge
      values:
        _: $totalNodes
  ```

  If `$totalNodes` is not found, now won't be logged at warn level.
  
  The same behavior is implemented for variables used in `loop` or `with_items` action.

#### BREAKING CHANGE

- remove feature that allows to set a single text not preceded by $ sign as value for key_labels or values:

```yaml
    - name: collect disks
      scope: results
      metrics:

        - metric_name: disk_status
          help: "physical disk status: 0: normal - 1: degraded - 2: New - 4: Failed - 99: Unknown"
          type: gauge
          key_labels:
            model: _
            serial: serialNumber # NOW FORBIDDEN
            position: cage-{{ .position.cage | default "undef" }}/Port-{{ .position.slot | default "undef" }}/diskPos-{{ .position.diskPos | default "undef" }}
            capacity: mfgCapacityGB # NOW FORBIDDEN
          values:
            _ : state # NOW FORBIDDEN
          loop: members # NOW FORBIDDEN
```

and is replaced by:

```yaml
- name: collect disks
  scope: results
  metrics:
    - metric_name: disk_status
      help: "physical disk status: 0: normal - 1: degraded - 2: New - 4: Failed - 99: Unknown"
      type: gauge
      key_labels:
        model: _
        serial: $serialNumber
        position: cage-{{ .position.cage | default "undef" }}/Port-{{ .position.slot | default "undef" }}/diskPos-{{ .position.diskPos | default "undef" }}
        capacity: $mfgCapacityGB
      values:
        _ : $state
      loop: $members
```

### 2025-03-23 - not release

- add trace infos for queries: the **query** action can now be set to enable traces collecting (disable by default), so that metrics can be created using that infos.

e.g.:

```yaml
scripts:
  check_status:
    - name: get status page
      query:
        url: /status
        var_name: results
        # debug: true
        parser: text-lines
        trace: true

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
        - metric_name: query_perf_seconds
          help: "query stage duration in seconds"
          type: gauge
          key_labels:
            stage: $item
            page: status
          values:
            _: $trace_infos.${item}
          with_items: '{{ exporterKeys .trace_infos | toRawJson }}'
```

then:

```text
# HELP query_perf_seconds query stage duration in seconds
# TYPE query_perf_seconds gauge
query_perf_seconds{page="status",stage="dns_lookup"} 0.00123
query_perf_seconds{page="status",stage="conn_time"} 0.00123
query_perf_seconds{page="status",stage="tcp_con_time"} 0.00123
query_perf_seconds{page="status",stage="tls_handshake"} 0.00123
query_perf_seconds{page="status",stage="server_time"} 0.00123
query_perf_seconds{page="status",stage="response_time"} 0.00123
query_perf_seconds{page="status",stage="total_time"} 0.00123
```

## 0.4.0 / 2025-03-22

### 2025-03-19 - not release

- bug fixed: synchronization error between target and collectors replies. target may stayed in infinite wait for collectors that had send replies too early in process (gofunc() channel synchro pb)
- added: debug messages (to track previous bug)
- fixed: json response format for /reload
  - {"message":"ok","status": 1,"data": {"reload": true}}

### 2025-03-16 - not release

- added: parser 'prometheus' that can interact with data returned by an exporter.
- added: documentation for [parsers](doc/parsers.md).
- fixed: debug message

### 2025-03-03 - not release

- added: parsers 'yaml' and 'text-lines' (split result into lines after "\r?\n"). See documentation [parsers](doc/parsers.md) for more infos.
- added: template function "exporterRegexExtract" to obtain matching group from regex and searching string: [array] <= exporterRegexExtract[regexp] [search_string]
- added: some gotest for exporter template function.

### 2025-02-17

- fixed: json response format for /status and /loglevel
  - {"message":"ok","status": 1,"data": {"status":"ok"}}
  - {"message":"ok","status": 1,"data": {"loglevel": "&lt;loglevel&gt;"}}

### 2025-01-15

- fixed: changed **--dry-run** command line flag behavior when no target specified: only check config; do not try to collect first available target.
- added: disable_warn: true|false in auth_config to disable warning messages from resty if auth is basic and connection is http.

  ```yaml
    auth_config:
      mode: basic
      user: ping
      password: ping
      disable_warn: true
  ```

### 2025-01-11

- fixed: config output in json format.
- fixed: warn messages ("script_name not defined"), displayed with loglevel "debug"

### 2025-01-08

- fixed: now checks that each target has at least one collector defined; e.g.: failed if collector pattern matching doesn't correspond to any collector name.
- fixed: allow "scope" directive to use $var_name format (like .var_name).

### 2025-01-04

- added "profile" for config and target. Now exporter can collect multiple apis with different "login" semantics. Before the config contains only one **httpapi_config** part that may define the "clear", "init", "login", "logout", "ping" scripts. The new version allows to define "profiles", and in each profile the "default scripts", so that a target can use a named profile :

  ```yaml
  profiles:
    default:
        # all metrics will be named "[metric_prefix]_[metric_name]"
        metric_prefix: "apiprefix"

        scripts:
          init:
            - name: default headers
              set_fact:
                scheme: https
                base_url: /my_url...
                verifySSL: true
                headers:
                  "Content-Type": application/json
                  Accept: "application/json"
          login: ~
          logout: ~

          # method call to determine if a target is responding; will call login() if necessary
          ping:
            - name: check if API is replying
              query:
                url: my_ping_url
                ...
  ```

  Alternatively it is possible to add profiles via profile files with **profiles_file_config** directive: set a list of filepath accepting wildcards '*' (golang fileglob ()) to "profiles". The content of each file must be a profile_config (see above or contribs)
  
  ```yaml
    profiles_file_config:
      - "*_profile.yml"
  ```

  Then each target may set a profile name to use; by default if not set, the exporter will try to assign the profile "default" or the only one if there is only one profile defined in configuration, else the config check will failed.
  Each profile, may also set a "metric_prefix", so that "up", "collector_status", "scrape_duration" metrics have a distinct name for each profile!

### 2024-12-14

- added: parameter "health" to endpoint "/metrics", so that only "ping" script is performed and the metrics "up" with status returned. May be useful to check if a target is responding; I use this feature in ansible playbook before to generate "file_sd_config" of scraping job for prometheus.
- added: parsers feature to decode response in the query action. parser can be:
  - `xml` for "text/xml" content : beta ; feedbacks are appreciated.
  - `json` default parser. Before 0.4.0 all replies must be in json format.
  - `none` for response that you don't want to parse (landing ping() page by example.)
  - `openmetrics` not implemented yet!

  httpapi_exporter can now provide unified results for multi-format api. I've got one that respond with both json and xml data.
  usage:

  ```yaml
    - name: collect elements
    query:
      url: /path/entrypoint
      var_name: results
      # debug: yes
      parser: xml

  ```

- fixed access to url without authentication (no user, password provided with default basic authentication). Removed the header generation (Authorization).
- allow dynamic metric name and help. Now it is possible to define metrics in a loop:

  ```yaml
  - metric_name: "config_{{ .counter.name }}"
    help: "parameter {{ .counter.value }} is enabled boolean (0: false - 1: true)"
    type: gauge
    key_labels: $labels
    values:
      _: "{{ convertBoolToInt .counter.value }}"
    loop:
      - name: hasAdvancedDocumentConversion
        value: $results.hasAdvancedDocumentConversion
      - name: hasAdvancedQueryReporting
        value: $results.hasAdvancedQueryReporting
    loop_var: counter
  ```

  that will produce:

  ```text
  # HELP exalead_license_config_hasAdvancedDocumentConversion parameter false is enabled boolean (0: false - 1: true)
  # TYPE exalead_license_config_hasAdvancedDocumentConversion gauge
  exalead_license_config_hasAdvancedDocumentConversion{company="My Company",param="hasSemanticFactory",type="-"} 0
  # HELP exalead_license_config_hasAdvancedQueryReporting parameter false is enabled boolean (0: false - 1: true)
  # TYPE exalead_license_config_hasAdvancedQueryReporting gauge
  exalead_license_config_hasAdvancedQueryReporting{company="My Company",param="hasSemanticFactory",type="-"} 0
  ```

  This example is a little bit stupid, because it is more accurate to add a label param with name of parameter in config metric, but it explains how it can work !
- add new template function `convertBoolToInt` to convert text boolean to value 0 or 1.
  - string "true", "yes", "ok" returns 1 anything else 0.
  - any int or float value distinct of 0 then 1 else 0.
  - map,slice: if length of map or slice is greater than 0 then 1 else 0.
- add contribs exalead exporter.

- add config parameters in configuration file in global section for:
  - web.listen-address (priority to config file over command line argument --web.listen-address)
  - log.level (priority to config file over command line argument --log.level)
  - up_help allow user to replace default help message for metric help (default is "if the target is reachable 1, else 0 if the scrape failed")
  - scrape_duration_help same for scrap duration metric (default is "How long it took to scrape the target in seconds")
  - collector_status_help same for collector status (default is "collector scripts status 0: error - 1: ok - 2: Invalid login 3: Timeout")

## 0.3.9 / 2024-12-14

- removed passwd_encrypt tool source code from httpapi_exporter: created a new stand-alone package [passwd_encrypt](https://github.com/peekjef72/passwd_encrypt). Passwd_encrypt is still installed when building and added to the released archive.
- updated prometheus/exporter-toolkit to 0.13.0 (log => log/slog)
- renamed entrypoint '/healthz' to /health : response format depends on "accept" header (application/json, text/plain, text/html default)
- updated entrypoint /status, /loglevel /targets /config: response format depends on "accept" header (application/json, text/plain, text/html default)
- added cmd line --model_name to perform test with model and uri in dry-run mode
- added out format for passwd_encrypt that can be cut/pasted into config file.
- added InvalidLogin error cases: no cipher (auth_key not provided) or (invalid auth_key). For those cases if target is up, metrics for collectors status will return code 2; invalid_login
- added GET /loglevel to retrieve current level, add POST /loglevel[/level] to set loglevel to level directly
- added debug message for basic auth (auth_config.mode=basic) and bearer (auth_config.mode=token)
- loglevel link in landing page
- fixed typos
- upgrade go version and modules, security fixed (golang.org/x/crypto)

## 0.3.8 / 2024-05-20

- fixed minor bug with basic auth, remove unused vars ...
- fixed typos.
- reorganized contribs dirs

### BREAKING CHANGES

- rename attribute "**auth_mode**" to **auth_config** in query_action and target definition:

  before:

  ```yaml
  targets:
    # default target is used as a pattern for exporter queries with target name not defined locally.
    - name: default
      scheme: https
      host: set_later
  =>    auth_mode:
        # mode: basic|token|[anything else:=> user defined login script]
        mode: script
        user: usrNetScalerSupervision
        password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
      collectors:
        - ~.*_metrics
  ```

  now:

  ```yaml
  targets:
    # default target is used as a pattern for exporter queries with target name not defined locally.
    - name: default
      scheme: https
      host: set_later
      auth_name: prometheus_encrypted
      auth_config:
        # mode: basic|token|[anything else:=> user defined login script]
        mode: script
        user: usrNetScalerSupervision
        password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
      collectors:
        - ~.*_metrics
  ```

- add POST /reload and /loglevel entry points to respectively do a reloadConfig and increase loglevel.
- build a specific windows code version without SIGUSR2 signal (used for loglevel cycling).

## 0.3.7 / 2024-04-21

- added support for env vars in auth_config [#1](https://github.com/peekjef72/httpapi_exporter/issues/1) : included from branch dev_issued_1
- upgraded to go 1.22.2
- upgraded to latest modules' version
- fixed cookie sessions (bug found with arubacx cnx)
- added contribs arubacx-os

## 0.3.6 / 2024-02-11

- upgraded to go 1.22
- upgraded to latest modules' version
  
## 0.3.5 / 2024-02-11

- fix panic when var is not found for metric
- fix target parsing when exporter is used in proxy mode: allow formats
  - target=host.domain : use default scheme and default port
  - target=host.domain:port : use default scheme
- added status value for collector_status metric :
  - 0: error
  - 1: ok
  - 2: invalid log
  - 3: timeout
- added new "template" format: $varname that allow a direct accept to variable in symbols table. it is easier to use this format for loop interaction.
  e.g.:

  ```yml
  loop: "{{ .item.list | toRawJson }}"
  ```

  can be replaced by:
  e.g.:

  ```yml
  loop: $item.list
  ```

- added a new template func "lookupAddr" to retrieve DNS hostname from ip address.
- adapt contribs (netscaler/veeam) with new features.
  
## 0.3.4 / 2023-12-16

- fixed var evaluation (set_fact with template)
- fixed type evaluation for cookies and header
- (beta) add set_stats action to store vars (and values) from collector into target global symbols table, so they are persistent across several runs; used to get config data only once or at periodic time.
- updated go version to 1.21.5
- updated contrib netscaler (lb services, ssl services, rename metrics from system collector)
- fixed template panic: add recover

## 0.3.3 / 2023-11-05

- add auth_key argument for cli in dry_mode
- add log.level cycling with signal USER2
- add Authconfig, dynamic targets
- fix logout with cookies set
- fix clear script calls
- fix global scrape timeout detection
- fix scrape timeout by target
- fix global cookies (always appended)
- config reload with signal HUP.
- minor bugfixes (log)

## 0.3.2 / 2023-10-19

- bugfixes

## 0.3.1 / 2023-09-24

- use standard prometheus args web.listen-address and web.config.file for https
- add server start_time in /status page
- modify http server routing process

## 0.3.0 / 2023-09-24

### Changed

- Initial release
