<!-- cSpell:ignore subactions, elmt, diskspace_free_bytes, mgmt, systemreporter, attime, cpustatistics -->
# Actions

Actions are the core of the exporter scripting features. They were designed to collect information from an HTTP site and then manipulate the received data to build metrics. The actions or commands have a deliberate resemblance to those of Ansible. We will therefore find the same operating logic.

All actions are gathered in a script; So a script is a list of actions, and to execute a script, means to launch each action in the order of the list.

## base command

Each command depends on the base command that have common features that can be use as desired. The base command is a loop of actions to execute. So each action can be a loop that execute its action as many time as specified; by default it is executed one time: single element loop.

- **name**: the name of the action, it is used to determine which action is run when the script is in debug mode.
- **vars**: a list of local variables to define for the execution of the command
- **when**: a boolean condition (or a list of conditions) to check before running the action: if condition is true action is run else not! By default if no **when** is specified action is run. If a list of conditions is specified all must be verified : a list of cond is equivalent to a one line cond separated by AND

- **until**: a boolean condition to check in loop context; the loop will continue until the condition is evaluated to false.
- **with_items** or **loop**: the list of element to loop on. If var is not a list, a temporary list containing the variable is used to loop on, so the loop will run only one time.
- **loop_var**: the name of the variable to use for the loop element, by default is it "**item**"

## block or subactions

- **actions**:

  Define a list of sub actions to perform. It is used to build a block of code for conditions, loops or metrics definitions.

  e.g: for a conditional loop

  ```yaml
  - name: proceed each elements from list
    with_items: $results
    loop_var: elmt
    when: >-
      js:
        elmt != undefined && elmt.fans != undefined && elmt.fans.length > 0
    actions:
      - name: loop fans
        with_items: $elmt.fans
        loop_var: fan
        actions:
          # - name: debug fan
          #   debug:
          #     msg: "fan {{ .fan }}"
          - name: build labels
            ...
  ```

- **metrics**:
  
  Define a list of metric_name actions.

  attributes:
  - metric_prefix: locally change the prefix for the metrics; by default used the metric_prefix set by priory order:
    - in global config
    - in collector config
    - metrics config

  - scope: restrict the symbols table to this map; this is used to ease the writing of formulae:
  
  e.g.:
  
    if you have the symbols table:

    ```yaml
    my_labels:
      key_label: value_label

    results:
      obj1:
        attr1-1: val11
        attr1-2: val12
      obj2:
        attr2-1: val21
        attr2-2: val22
      ...
    ```

    and want to build a metric on the value 'val11', without scope you have to use :

    ```yaml
    - name: metrics def
      metrics:
        - metric_name: my_metric
          type: gauge
          value: $results.obj1.attr1-1
    ```

    and with scope:

    ```yaml
    - name: metrics def
      scope: $results.obj1
      metrics:
        - metric_name: my_metric
          type: gauge
          help: my help
          value: $attr1-1
    ```

    The main symbols table stays available by the 'root' entry (only in a scoped context):

    ```yaml
    - name: metrics def
      scope: $results.obj1
      metrics:
        - metric_name: my_metric
          type: gauge
          help: my help
          key_labels: $root.my_labels
          value: $attr1-1
    ```

## core actions

### set_fact

The **set_fact** action allows to set variables; the syntax is `var_name: var_value`.

```yaml
- name: set variable
  set_fact:
    var_name: var_value # single var
    var_name_2: # var contains a list
      - var_value_2_1
      - var_value_2_2
      - var_value_2_3
    var_name_3: # var contains a map
      - name:  value
        attrs: attrs_val
```

In this example, the value is simply a text value, but it should be a map, a list, a go template ({{ .var }}) or exporter variable reference notation ($var)

### debug action

The **debug** action allows to print message to log when the exporter's log level is set to debug.
This action has only one attribute: **msg**; the value set to msg is a text to display; it is usually a go template containing the text and variables to display.

e.g.:

```yaml
- name: debug fan
  debug:
    msg: "fan {{ .fan }}"
```

but it should be a loop:

```yaml
- name: debug fan
  debug:
    msg: "fan {{ .fan }}"
  with_items: [1, 2, 3]
  loop_var: fan
```

In this stupid example, we build a loop with 3 elements 1,2,3 then we loop on each, we define that the loop element is called **fan**, and we ask to display a debug message that is a go template.

### query

#### attributes

- **url**:
- **method**: GET, POST ...
- **data**: the data to send with the request.
- **debug**: boolean value to indicate to log information on the query sent and the results of the query.
- **var_name**: name of the variable to store the results of the parser
- **ok_status**: the http server status code to consider that the response is OK: default is 200
- **auth_config**: a specific auth_config to use for the query if different from global one.
- **timeout**: integer value second, specific timeout overwrite the global value
- **parser**: parser to use to read the response from sever. See [parsers.md](parsers.md).
- **trace**: boolean value to indicate to collect performance data from query. If the connection is successful it will add a map variable **trace_infos** in symbols table containing:
  - **dns_lookup**: is a float64 fractional number representing the duration in seconds that transport took to perform DNS lookup.
  - **conn_time**: is a float64 fractional number representing the duration in seconds it took to obtain a successful connection.
  - **tcp_con_time**: is a float64 fractional number representing the duration in seconds it took to obtain the TCP connection.
  - **tls_handshake**: is a float64 fractional number representing the duration in seconds of the TLS handshake.
  - **server_time**: is a float64 fractional number representing the duration in seconds for responding to the first byte.
  - **response_time**: is a float64 fractional number representing the duration in seconds since the first response byte from the server to request completion.
  - **total_time**: is a float64 fractional number representing the duration in seconds of the total time request taken end-to-end.

### metric_name

Define a metric family.

Attributes:

- **name** (metric_name): the name of the metric family; final name is prefixed by metric_prefix.

- **type** (mandatory): gauge or counter or histogram

- **help**: a help text associated with the metric; don't forget to mention the unit of the value if not specified in the name. It is much easier to build a dashboard to know that !

- **static_labels**: constant labels key/value pairs to add to the metrics.

- **key_labels**: represents the variation on external criteria for the value; classical example is the partition or mount point name for free disk space bytes: we have a single metric name **diskspace_free_bytes** labeled by each partition (see [example below](#key_labels-example)). You can set the value of the key/value pair to special '_' meaning the corresponding key value in symbols table.

- **value_label**: represents a variation on the value itself: difference between previous (key_labels) is weak and depends on the way the value is collected. Is is used when we want to gather into a single metric name the different collected values. (see [example below](#value_label-example))
- **scope**: like for the `metrics` action, it restricts the symbols table to this map; this is used to ease the writing of formulae. **Remarks**: **be careful that in a loop context, the scope is default to `loop_var`**.
- **values**: the value or values to use for the metric. A map[string]string mus to specified.

- **histogram**: specific definitions for histogram metrics (see [histograms](histogram.md))

#### **key_labels** example

We have collected data and store the results in a variable called `results` that should contain:

```json
[
    {
        "comm_ip_addr": "172.17.17.100",
        "comm_mac_addr": "bc:d7:a5:cc:ce:01",
        "hardware_revision": "",
        "is_local": true,
        "mgmt_module": "/rest/v1/system/subsystems/management_module/1%2F1",
        "mgmt_role": "Active",
        "name": "1/MM1",
        "remote_present": false,
        "software_revision": "",
        "state": "ready"
    }
    , ...
]
```

The collected data represent several information of a management module (aruba switch) identified by its name.

To build a metric named `management_module_status` representing their status for each module, you have to consider the status value as a value, and the module name as a label for the status.
It this case, the label - the module name - is obtained directly, but the value that is in plain text in the data must be converted to a numeric value; for that purpose we build a small javascript that checks the module status string and return an integer 0,1 or 2.

```yaml
    - name: proceed each elements from list
      with_items: $results
      loop_var: mod
      actions:
        - name: management module status
          # by default scope is set to loop_var, here $mod; because we need $key_labels var
          # scope must be set to none
          scope: none
          metrics:
            - metric_name: management_module_status
              help: "management module status: 0: not ok / 1: ready / 2: empty"
              type: gauge
              key_labels:
                name: $mod.name
              values:
                _: >-
                  js:
                    var res = 0
                    switch( mod.state ) {
                      case "ready":
                        res = 1
                        break
                      case "empty":
                        res = 2
                        break
                    }
                    res
```

We will obtain:

```text
# HELP management_module_status management module status: 0: not ok / 1: ready / 2: empty
# TYPE management_module_status gauge
management_module_status{name="1/MM1"} 1
```

#### **value_label** example

We have collected data and store the results in a variable called `results` that should contain

```json
{
    "sampleTime": "2022-11-10T18:25:00+01:00",
    "sampleTimeSec": 1668101100,
    "total": 24,
    "members": [
      {"node":0, "cpu":0, "userPct": 1.8, "systemPct":8.5, "idlePct":89.7, "interruptsPerSec":40452.9, "contextSwitchesPerSec":84915.9},
      {"node":0, "cpu":1, "userPct":1.2, "systemPct":24.8, "idlePct":73.9, "interruptsPerSec":0.0, "contextSwitchesPerSec":0.0},
       ...
    ]
}
```

The collected data represent the cpu usage by type, node and cpu.

To build a metric named `cpu_usage_percent` for each node, cpu, and type you have to consider two things:

- the value: it is represented by the value from several modes : **userPct**, **systemPct** and **idlePct**, so it is not a classical case of labels, but a value label. So we have to define a name for the value variation, here **mode**, and a value for each mode; in practice it means that when value is "userPct" we want to add a label pair **mode**: **user**, when value is **systemPct** => **mode**: **system**, and so on.

- the classical variation of the value, in this case **node** and **cpu** are key_labels.

With the below code:

```yaml
  get system_cpu_stats:
    - name: query system_cpu_stats
      query:
        url: /systemreporter/attime/cpustatistics/hires
        var_name: results
        debug: yes
    - name: collect data cpu
      scope: results
      metrics:
        - metric_name: cpu_usage_percent
          help: cpu percent usage over last 5 min for system, user and idle (labeled mode) by node and cpu core
          type: gauge
          key_labels:
            node: _
            cpu: _
          value_label: mode
          values:
            user:   $userPct
            system: $systemPct
            idle:   $idlePct
          loop: $members
```

We will obtain:

```text
# HELP cpu_usage_percent cpu percent usage over last 5 min for system, user and idle (labeled mode) by node and cpu core
# TYPE cpu_usage_percent gauge
cpu_usage_percent{cpu="0",mode="idle",node="0"} 89.7
cpu_usage_percent{cpu="0",mode="system",node="0"} 8.5
cpu_usage_percent{cpu="0",mode="user",node="0"} 1.8
cpu_usage_percent{cpu="1",mode="idle",node="0"} 73.9
cpu_usage_percent{cpu="1",mode="system",node="0"} 24.8
cpu_usage_percent{cpu="1",mode="user",node="0"} 1.2
```

#### histogram example

### play_script

### set_stats
