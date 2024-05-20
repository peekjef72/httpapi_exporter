# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
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
