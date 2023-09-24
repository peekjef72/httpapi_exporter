# Prometheus HTTPAPI Exporter

This exporter wants to be a generic JSON REST API exporter. That's mean it can login, then makes requests to collect metrics and performs transformations on values and finally returns metrics in prometheus format.

Nothing is hard coded in the exporter. That why it is a generic exporter.

As examples 3 configurations for exporters are provided (see contribs):
- hp3par_exporter
- veeam_exporter
- netscaler_exporter

