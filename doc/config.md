
# Configuration

```yaml
# Global defaults.
global:
  scrape_timeout: 30s
  # Subtracted from Prometheus' scrape_timeout to give us some headroom and prevent Prometheus from timing out first.
  scrape_timeout_offset: 500ms
  # Minimum interval between collector runs: by default (0s) collectors are executed on every scrape.
  min_interval: 0s
  # all unsuccessful queries will be retried this number of times
  query_retry: 3
  # all metrics will be named using the formula "[metric_prefix]_[metric_name]"
  metric_prefix: <global_prefix>
  # all http codes that will be consider auth is invalid and must do a Login()
  invalid_auth_code: [401,403]
  exporter_name: <name_exporter>
  # list of allowed tls version, meaning authorized ciphers for https connections
  #   all, tls_upto_1.2, tls_1.2 ,tls_1.3
  # tls_version: "tls_upto_1.2,tls_1.2,tls_1.3" or "all"

#
profiles: # list of profile_configs
  # profile_config definition : map of profile names with  metric_prefix and scripts mapping.
  <profie_name>:
    # all metrics of the profile will be named using the formula "[metric_prefix]_[metric_name]"
    metric_prefix: <global_prefix> # optional

    # dictionnary of scripts definitions to handle connections to REST API
    # some script names are use internally to perform actions:
    # "init": script used to initialize connections parameters like headers, vars, etc...
    # "login": script used if connection to the API requires a login phase; by e.g. to obtain a token after sending login/password.
    # "logout": if you want to logout after each scrapping
    # "clear": to reset previously set values.
    # "ping": script called to determine if a target is UP (responding); it would call login script if necessary
      # depending on resulting http code received : invalid_auth_code ( set in global.invalid_auth_code they are http 401, 403 usually ) on each query
    # only the "ping" script is mandatory
    scripts:
      init:
        - name: default headers
          # allowed/recognized vars that are internally :
          # scheme http, https...
          # host:
          # port:
          # base_url: base_path that will prefix all url send to API
          # proxy_url:
          # verifySSL: true|false 
          # queryRetry: update 
          # headers: map
          # cookies: map
          # if you set them in init script they will overwrite the values specified in GlobalConfig or in target config
          #
          # you can set your own vars here too, if they are usefull somewhere else in your scripts later...
          set_fact:
            session_id: ''
            headers:
              - name: "Content-Type"
                value: application/json
              - name: Accept
                value: "application/json"
            base_url: /nitro/v1
            verifySSL: true

      # script called to determine if a target is responding; it will call login script if necessary
      # it will generate <metric_prefix>_up metric according to the server response.
      # new login phase is determined by http code received : invalid_auth_code ( http 401, 403 usually ) on each query
      ping:
        - name: check ping cnx
          query: 
            # will call base_url + url in fact !
            url: /config/nsversion
            method: get
            ok_status: 200
            # to catch and store the results (content object) and use it later...
            var_name: nsversion

      ## login script example:
      # it will loop until queryRetry times (or timeout) :
      #  - to try to POST to /config/login url the data `{"login": {"username":"<login>", "password":"<password>"}}`
      #  - analyze the result status:
      #    if code == 201: set cookie session_id with value from response (query.var_name : login => .login.session_id)
      #    else if code != 401 && != 403: set loop_var (login_retry) to max(queryRetry) to leave the loop immediately 
      #    else increase value of loop_var (login_retry) to loop on
      login:
        - name: init login loop
          vars:
            login_retry: 0
            results_status: 0
          until: "{{ $login_retry := .login_retry | int }}{{ lt $login_retry .queryRetry }}"

          actions:
            - name: login phase
              query:
                url: /config/login
                method: post
                # obtain decrypted password from encrypted one from config
                #  (.password) and from query parameter auth_key (.auth_key)
                # build a dict obj $login = { "username": "<user>", "password": "<decrypted_password>"}
                # build a dict obj $data = { "login": $login }
                # return json reprensation of $data as data of login query
                data: >-
                  {{ $tmp_pass := exporterDecryptPass .password .auth_key }}
                  {{ $login := dict "username" .user "password" $tmp_pass }}
                  {{ $data := dict "login" $login }}
                  {{ $data | toRawJson }}
                # tell the query command that code 201 is ok (default ok code is 200)
                # if code is not OK, meaning result_status is not in ok_status list, the query status is false, and the query will be retried.
                ok_status: 201
                # store the result object in variable "login", so we can handle value in "login" object.
                var_name: login
            # play the "auth_check" script to analyse query
            # the actions should also be set here instead of in an another script...
            - name: analyze login response
              play_script: auth_check

      auth_check:
        - name: analyze login response ok
          set_fact:
            cookies: 
              - name: "sessionid"
                value: '{{ .login.sessionid }}'
            logged: true
            login_retry: "{{ .queryRetry }}"
          when:
            - eq .results_status 201
        - name: analyze login response
          set_fact:
            logged: false
            login_retry: "{{ .queryRetry  }}"
          when:
            - or (eq .results_status 401) (eq .results_status 403)
        - name: analyze login response not ok with retry
          set_fact:
            logged: false
            login_retry: "{{ add .login_retry 1 }}"
          when:
            - and (and (ne .results_status 201) (ne .results_status 401)) (ne .result_status 403)

# alternative to add profiles via config files: set a list of filepath accepting wildcards '*' (golang filegob ()) to "profiles".
# the content of each file must be a profile_config (see above or contribs)
profiles_file_config:
  - "*_profile.yml"

# OBSOLETE
# define the default scripts : if still present it will generate a profile "default" with scripts.
httpapi_config:
  # ping:
  #   - name:
  #     ...
  # login:
  #   - name:
  #     ...

# Collector files specifies a list of globs. One collector definition is read from each matching file.
# so the collector file contains only one collectore_config (see below)
collector_files:
  - "metrics/*.collector.yml"

# collectors: list of collector that defines what to do to collect and define metrics
collectors:
  # list of collector_config
    # mandatory collector_name
  - collector_name: <name>
    # optional metric prefix for that specific collector, generally a sub section of export name.
    metric_prefix: <global_prefix>_<name>

    # optional dictionnary of go templates definition used by this collector.
    # here templates are used as "function" to transform values
    templates:
      # e.g. define masterState template; it translates a map of label to a corresponding (string) value, that can be used later in process
      # masterState: '
      #   {{- $masterStateDef := dict
      #         "PRIMARY"       "1"
      #         "SECONDARY"     "2"
      #         "STAYSECONDARY" "3"
      #         "CLAIMING "     "4"
      #         "FORCE CHANGE"  "5"
      #   }}
      #   {{ pluck . $masterStateDef | first | default "0" }}'
      # then :
      # - name: my usage
      #   set_fact:
      #     hastate : '{{- template "masterState" ( .node.state | upper) -}}'

    # a dictionnary of script to perform.
    # a script is a list of actions to perform for the collector:
    # it is generally a query on a specific url of the API, then an analizis of the results an a format on them.
    # at leat one acion is necessary
    scripts:
      # see contribs for working example

# optional dictionnary of authentication parameters (name: AuthConfig)
# they can be used in local target definitions, or used in dynamic target scrapping
# user, password or token can be set to use system environment variable
#  e.g.:
#  user: $env:EXPORTER_USER
#  password: $env:EXPORTER_PASSWD

auth_configs:
  name_entry_1:
    # mode: basic|token|[anything else:=> user defined login script]
    mode: script
    user: <login>
    password: <password>

  name_entry_2:
    mode: basic
    user: <login>
    password: /encrypted/<encryped_password>
    # allow to disable warning messages from RESTY if auth is basic and connection is http.
    disable_warn: <true>

  name_entry_3:
    mode: token
    token: <bearer token>

  # use this auth_config to authenticate via env vars
  name_entry_4:
    mode: script
    user: $env:VEEAM_EXPORTER_USER
    password: $env:VEEAM_EXPORTER_PASSWD

# The targets to monitor and the collectors to execute on it.
targets:
  # target "default" is used as a pattern for all targets name not defined locally. => exporter is used in "proxy" mode.
  # list of target_config-s

  - name: default
    scheme: https
    host: template
    port: 443
    # if target needs to authenticate, define here the auth params
    # either use an auth_name or a statically defined params:
    # auth_name: default
    # or has its own auth paramaters
    # auth_config:
    #   # mode: basic|token|[anything else:=> user defined login script]
    #   mode: <mode>
    #   user: <login>
    #   password: <password>
    # profile: <profile_name> # by default use "default" or 

    # list of collector names (not collector file names!) to compute for the target.
    # it should be a exact name or the regexp pattern
    # ~<pattern>: all collector names matching the pattern (include)
    # !~<pattern>: all collector names not matching the pattern (exclude)
    collectors:
      - ~.*_metrics

# others targets
  - name: <target>
    ...

  # or use a specific list of files (fileglob) for each target definition (easier way to active/remove a single target)
  # each file define a target it the format target_config (see above).
  - targets_files: [ "/etc/httpapi_exporter/netscaler/targets/*.yml" ]

```
