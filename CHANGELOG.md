# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
## 0.3.7 / 2024-02-17
- add support for env vars in auth_config [#1](issues/1)

## 0.3.6 / 2024-02-11
- upgrade to go 1.22
- upgrage modules version
  
## 0.3.5 / 2024-02-11
- fix panic when var is not found for metric
- fix target parsing when exporter is used in proxy mode: allow formats 
  - target=host.domain : use default scheme and default port
  - target=host.domain:port : use default scheme
- add status value for collector_status metric :
  - 0: error
  - 1: ok
  - 2: invalid log
  - 3: timeout
- add new "template" format: $varname that allow a direct accept to variable in symbols table. it is easier to use this format for loop interaction.
  e.g.:
  ```
  loop: "{{ .item.list | toRawJson }}"
  ```
  can be replaced by:
  e.g.:
  ```
  loop: $item.list
  ```
- add a new template func "lookupAddr" to retrive DNS hostname from ip address.
- adapt contribs (netscaler/veeam) with new features.
  
## 0.3.4 / 2023-12-16
 - fix var evaluation (set_fact with template)
 - fix type evalution for cookies and header
 - (beta) add set_stats action to store vars (and values) from collector into target global symbols table, so they are persistent accross several runs; used to get config datas only once or at periodic time.
 - update go version to 1.21.5
 - update contrib netscaler (lb services, ssl services, rename metrics from system collector)
 - fix template panic: add recover

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
