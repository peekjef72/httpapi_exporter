# Prometheus HTTPAPI Exporter

This exporter wants to be a generic JSON REST API exporter. That's mean it can login, then makes requests to collect metrics and performs transformations on values and finally returns metrics in prometheus format.

Nothing is hard coded in the exporter. That why it is a generic exporter.

As examples 3 configurations for exporters are provided (see contribs):
- hp3par_exporter
- veeam_exporter
- netscaler_exporter

# build

## use promu tool

```shell
go install github.com/prometheus/promu@latest

$GOBIN/promu build

```
this will build the exporter and a tool to crypt/decrypt ciphertext with a shared passphrase.

## usage

```shell
usage: httpapi_exporter [<flags>]


Flags:
  -h, --[no-]help          Show context-sensitive help (also try --help-long and --help-man).
      --web.listen-address=":9321"  
                           The address to listen on for HTTP requests.
      --web.telemetry-path="/metrics"  
                           Path under which to expose collector's internal metrics.
  -c, --config.file="config/config.yml"  
                           Exporter configuration file.
  -n, --[no-]dry-run       Only check exporter configuration file and exit.
  -t, --target=TARGET      In dry-run mode specify the target name, else ignored.
  -m, --metric=METRIC      Specify the collector name restriction to collect, replace the collector_names set for each target.
      --log.level=info     Only log messages with the given severity or above. One of: [debug, info, warn, error]
      --log.format=logfmt  Output format of log messages. One of: [logfmt, json]
  -V, --[no-]version       Show application version.
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

- set the user password in the target file:

    ```yaml
    name: hp3parhost
    scheme: https
    host: "1.2.3.4"
    port: 8080
    baseUrl: /api/v1
    auth_mode:
    # mode: basic(default)|token|[anything else:=> user defined login script]
    user: prometheus
    # password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
    password: /encrypted/CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=
    ```
- set the shared passphrase in prometheus config (either job or node file)

    ```yaml
    - targets: [ "hp3par_node_1" ]
    labels:
        __tmp_source_host: "hp3par_exporter_host.domain.name:9321"
    # if you have activated password encrypted passphrass
        __auth_key: 0123456789abcdef
        host: "hp3par_node_1_fullqualified.domain.name"
        #custom labels…
        environment: "DEV"
    ```
