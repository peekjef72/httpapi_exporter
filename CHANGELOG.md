# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
## 0.4.0 / 2025-03-03 - not release

### 2025-03-03 - not release
- added: parsers 'yaml' and 'text-lines' (split result into lines after "\r?\n")
- added: template function "exporterRegexExtract" to obtain matching group from regex and searching string: [array] <= exporterRegexExtract[regexp] [search_string]

### 2025-02-17
- fixed: json reponse format for /status and /loglevel
  - {"message":"ok","status": 1,"data": {"status":"ok"}}
  - {"message":"ok","status": 1,"data": {"loglevel": "&lt;loglevel&gt;"}}

###  2025-01-15
- fixed: changed **--dry-run** command line flag behavior when no target specified: only check config; do not try to collect first available target.
- added: disable_warn: true|false in auth_config to disable warning messages from RESTY if auth is basic and connection is http.

  ```yaml
    auth_config:
      mode: basic
      user: ping
      password: ping
      disable_warn: true
  ```

###  2025-01-11
- fixed: config output in json format.
- fixed: warn messages ("script_name not defined"), displayed with loglevel "debug"

###  2025-01-08
- fixed: now checks that each target has at least one collector defined; e.g.: failed if collector pattern matching doesn't correspond to any collector name.
- fixed: allow "scope" directive to use $var_name format (like .var_name).

###  2025-01-04
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

  Alternatively it is possible to add profiles via profile files with **profiles_file_config** directive: set a list of filepath accepting wildcards '*' (golang filegob ()) to "profiles". The content of each file must be a profile_config (see above or contribs)
  ```yaml
    profiles_file_config:
      - "*_profile.yml"
  ```

  Then each target may set a profile name to use; by default if not set, the exporter will try to assign the profile "default" or the only one if there is only one profile defined in configuration, else the config check will failed.
  Each profile, may also set a "metric_prefix", so that "up", "collector_status", "scrape_duration" metrics have a distinct name for each profile!

###  2024-12-14

- added: parameter "health" to endpoint "/metrics", so that only "ping" script is performed and the metrics "up" with status returned. May be usefull to check if a target is responding; I use this feature in ansible playbook before to generate "file_sd_config" of scraping job for prometheus.
- added: parsers feature to decode response in the query action. parser can be:
  - `xml` for "text/xml" content : beta ; feedbacks are appreciated.
  - `json` default parser. Before 0.4.0 all replies must be in json format.
  - `none` for reponse that you don't want to parse (landing ping() page by example.)
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

- fixed access to url without authentication (no user, password provided with defaut basic authentication). Removed the header generation (Authorization).
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
  this example is a lilte bit stupid, because it is more accurate to add a label param with name of parameter in config metric, but it explains how it can work !
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
- removed passwd_encrypt tool source code from httpapi_exporter: created a new stand-alone package [passwd_encrypt](https://github.com/peekjef72/passwd_encrypt). Passwd_encrypt is still installed when building and added to the released archiv.
- updated prometheus/exporter-toolkit to 0.13.0 (log => log/slog)
- renamed entrypoint /healthz to /health : response format depends on "accept" header (application/json, text/plain, text/html default)
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

- added a new template func "lookupAddr" to retrive DNS hostname from ip address.
- adapt contribs (netscaler/veeam) with new features.
  
## 0.3.4 / 2023-12-16

- fixed var evaluation (set_fact with template)
- fixed type evalution for cookies and header
- (beta) add set_stats action to store vars (and values) from collector into target global symbols table, so they are persistent accross several runs; used to get config datas only once or at periodic time.
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
