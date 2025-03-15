<meta name="google-site-verification" content="W1W5S9kNhNL2ZcTEZJ6lMZpAGeNnka6I3iIVFhiUO-I" />

# Prometheus HTTPAPI Exporter

This exporter wants to be a generic JSON REST API exporter. That's mean it can login, then makes requests to collect metrics and performs transformations on values and finally returns metrics in prometheus format.

Nothing is hard coded in the exporter. That why it is a generic exporter.

As examples 4 configurations for exporters are provided (see contribs):
- [hp3par_exporter](contribs/hp3par/README.md)
- [veeam_exporter](contribs/veeam/README.md)
- [netscaler_exporter](contribs/netscaler/README.md)
- [aruba_exporter](contribs/arubacx/README.md)

# Build

## use promu tool

```shell
go install github.com/prometheus/promu@latest
go install github.com/peekjef72/passwd_encrypt@latest

$GOBIN/promu build
```

or simply make.

```shell
make
```

this will build the exporter and a tool to crypt/decrypt ciphertext with a shared passphrase.

## Usage

```text
usage: httpapi_exporter [<flags>]


Flags:
  -h, --[no-]help            Show context-sensitive help (also try --help-long and --help-man).
      --web.listen-address=":9321"
                             The address to listen on for HTTP requests.
      --web.telemetry-path="/metrics"
                             Path under which to expose collector's internal metrics.
  -c, --config.file="config/config.yml"
                             Exporter configuration file.
  -n, --[no-]dry-run         Only check exporter configuration file and exit.
  -t, --target=TARGET        In dry-run mode specify the target name, else ignored.
  -a, --auth.key=AUTH.KEY    In dry-run mode specify the auth_key to use, else ignored.
  -o, --collector=COLLECTOR  Specify the collector name restriction to collect, replace the collector_names set for each target.
      --log.level=info       Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt    Output format of log messages. One of: [logfmt, json]
  -V, --[no-]version         Show application version.
```

## Loging level

You can change the log.level online by sending a signal USR2 to the process. It will increase and cycle into levels each time a signal is received.

```shell
kill -USR2 pid
```
Usefull if something is wrong and you want to have detailled log only for a small interval.

You can also set the loglevel using API endpoint /loglevel
* GET /loglevel : to retrieve the current level
* POST /loglevel : to cycle and increase the current loglevel
* POST /loglevel/\<level\> : to set level to \<level\>

## Reload

You can tell the exporter to reload its configuration by sending a signal HUP to the process or send a POST request to /reload endpoint.

## Exporter configuration

Exporter requires configuration to works:
- globals parameters
- collectors
- targets
- authentication definitions
  
  see [config.md](doc/config.md) documentation

## exporter http server

the exporter http server has a default landing page that permit to access
* "/health" : a simple heartbeat page that return "OK" if exporter is UP
* "/configuration": expose defined configuration of the exporter
* "/targets": expose all known targets (locally defined or dynamically defined). Password are masked.
* "/status": expose exporter version, process start time
* "/profiling": expose exporter debug/profiling metrics
* "/httpapi_exporter_metrics": exporter internal prometheus metrics
* "/help: help on github.
* "/metrics": expose target's metrics.
* "/loglevel": GET exposes exporter current log level. POST /loglevel increases by one the current level (cycling). POST /loglevel/[level] set the new [level].
* "/reload": method POST only: tells the exporter to reload the configuration.

## exporter metrics access

parameters to scrape a target:

* target: `<locally_defined_target>` or `<scheme://[user:password@]host:port>` (dynamic target)
* auth_key: the shared secret key used to decrypt encrypted password set in authentication config.
* auth_name: the name of authentication config to use to access to a target.
  if target is not defined locally (so it is dynamically defined), you can set the authentication parameters to use for that target using those specified in the auth_name config.
* model: the name of model target to use to build dynamic target. If not specified it looks for target named "default". This parameter is used only at the first call for the dynamic target creation.

**examples**:

1. `/metrics?target=mytarget` scrapes the target `mytarget` without any parameter; It must be fully defined in the exporter configuration files; it has either no authentication or password is not encrypted.
2. `/metrics?auth_key=<cipherkey>&target=mytarget2` scrapes the target `mytarget2` that is fully defined in the exporter configuration files, and has a password that is encrypted and can be decrypted with the cipherkey.
3. `/metrics?auth_key=<cipherkey>&target=<https://myhost.domain.name:port>&auth_name=<auth_name>` define a dynamic target that is reachable at url `https://myhost.domain.name:port`, using the authentication parameters defined by `<auth_name>` and "default" model, then scrapes it. This target was not initially defined in the exporter configuration files, and only exists until the exporter is running.
4. `/metrics?auth_key=<cipherkey>&target=<https://myhost.domain.name:port>&auth_name=<auth_name>&model=mytarget` same than previous example but the dynamic target creation use "mytarget" model instead of default.

## Authentication

Most of the time the access to a http api requires an authentication. It is the case for the 3 contribs (hp3par, veeam, netscaler).
The exporter allows you 2 modes:
 - to define the authentication parameters statically for each target.
 - to define a dictionnary (map) of authentications and then to use it a target definition, or even to set it in the prometheus query.

The auth parameters are :
  - mode: should be 
    - basic: use http basic authentication: the "user" and "password" values will be sent in the http header request
    - token: use bearer token to authenticate
    - script: user, password will be used in the login script to access the api.
  - user
  - password
  - token

The user, password and token values can be raw strings or retrive the values from environment variables. To use env var the value must be prefixed by "$env:" followed by the name of the variable.

```yml
  # use this auth_config to authenticate via env vars
  default:
    mode: script
    user: $env:VEEAM_EXPORTER_USER
    password: $env:VEEAM_EXPORTER_PASSWD
```

## password encryption

If you don't want to write the users' password in clear text in config file (targets files on the exporter), you can encrypt them with a shared password.

How it works:
- choose a shared password (passphrase) of 16 24 or 32 bytes length and store it your in your favorite password keeper (keepass for me).
- use passwd_encrypt tool:

    ```bash
    ./passwd_encrypt 
    give the key: must be 16 24 or 32 bytes long
    enter key: 0123456789abcdef 
    enter password: mypassword
    Encrypting...
    Encrypted message hex: CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=
    $
    ```

- set the user password in the target file or in auth_configs part:

    ```yaml
    name: hp3parhost
    scheme: https
    host: "1.2.3.4"
    port: 8080
    baseUrl: /api/v1
    auth_config:
      # mode: basic(default)|token|[anything else:=> user defined login script]
      user: <user>
      # password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
      password: /encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=
    ```

    or

    ```yaml
    auth_configs:
      <auth_name>:
        # mode: basic|token|[anything else:=> user defined login script]
        mode: <mode>
        user: <user>
        # password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
        password: /encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=

    ```

- set the shared passphrase in prometheus config (either job or node file)

  * prometheus jobs with target files:
    ```yaml
    #--------- Start prometheus hp3par exporter  ---------#
    - job_name: "hp3par"
        metrics_path: /metrics
        file_sd_configs:
        - files: [ "/etc/prometheus/hp3par_nodes/*.yml" ]
        relabel_configs:
        - source_labels: [__address__]
            target_label: __param_target
        - source_labels: [__tmp_source_host]
            target_label: __address__

    #--------- End prometheus hp3par exporter ---------#
    ```

    ```yaml
    - targets: [ "hp3par_node_1" ]
    labels:
        __tmp_source_host: "hp3par_exporter_host.domain.name:9321"
    # if you have activated password encrypted passphrass
        __param_auth_key: 0123456789abcdef
        host: "hp3par_node_1_fullqualified.domain.name"
        # custom labels…
        environment: "DEV"
    ```
# custom functions for templating
 
(**still incomplete**)

Go template is used to manipulate data. So templates inherits from go functions and [sprig](https://masterminds.github.io/sprig/) v3 functions.
Because the exporter uses most of the time data of type "any (interface{})" some of the sprig functions failed.
Here is the list of exporter functions:

name| usage | e.g. |
|---|---|---|
exporterDecryptPass | |
exporterGet [varmap] [keyname] | get the keyname from the map. Like sprig/get function but accepts data of type map[any]any | exporterGet .svclb .svc
exporterSet | |
exporterKeys | |
exporterValues | |
exporterToRawJson | |
|||
lookupAddr [varstring] | obtain hostname from string representing an ip address ; like sprig/getHostByName but for string ip | lookupAddr .node.ipaddress
convertToBytes value unit | convert the value contained in variable to bytes according to the unit string specified: <ul><li>"kilobyte" or "Kb" multiply by 1024 <li>"megabyte" or "Mb" multiply by 1024 * 1024<li>"gigabyte" or "Gb"multiply by 1024 * 1024 * 1024</ul> | '{{ convertToBytes .result.totalMiB "Mb" }}'
convertBoolToInt value | convert value that may contain a boolean to 0&#124;1 representation. Value can be of any type. If something is <ul><li>like int or float and different from 0 is 1 else 0<li>string and is lower case 'true' or 'yes' or 'ok' is 1 else 0<li>like map or array and length >0 then 1 or 0</ul>| with {"proc": {"loopCrashing": "true",...}}<br> => '{{ convertBoolToInt .proc.loopCrashing }}<br> => 1'
getHeader [varmap] | |
LEN [var]| obtain the len of the var. works like sprig/len but accepts data of type any. |
exporterRegexExtract [regexp var] [search var] : []string | obtain the list of extracted found element from regexp on search | extract value from line as group 1 fof regexp: <br> res: "{{ index  (exporterRegexExtract "^status:(.*+)" "status:OK") 1 }}"

## boolean checks
name| usage | e.g. |
|---|---|---|
EQ [var1] [var2] | check equality for 2 variables; accepts any type of data; meaning that the second will be converted to the type of the first | EQ .val "2"
NE [var1] [var2] | not equal | NE 2 .val
GE [var1] [var2] | greater equal |
GT [var1] [var2] | greater than |
LE [var1] [var2] | less equal |
LT [var1] [var2] | less than |
exists [var1]| return boolean if variable exists| exists .config.cluster.node
exporterHasKey [var] [key]| check if variable is a map and has a key | exporterHasKey .config "cluster"
