# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/) and [Keep a changelog](https://github.com/olivierlacan/keep-a-changelog).

 <!--next-version-placeholder-->
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
