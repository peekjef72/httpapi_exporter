# Actions

Actions are the core of the exporter scripting features. They were designed to collect information from an HTTP site and then manipulate the received data to build metrics. The actions or commands have a deliberate resemblance to those of Ansible. We will therefore find the same operating logic.

All actions are gathered in a script; So a script is a list of actions, and to execute a script, means to launch each action in the order of the list.

## base command

Each command depends on the base command that have common features that can be use as desired. The base command is a loop of actions to execute. So each action can be a loop that execute its action as many time as specifed; by default it is executed one time: single element loop.

- **name**: the name of the action, it is used to determine which action is run when the script is in debug mode.
- **vars**: a list of local variables to define for the execution of the command
- **when**: a boolean condition (or a list of conditions) to check before running the action: if condition is true action is run else not! By default if no **when** is specified action is run. If a list of conditions is specified all must be verified : a list of cond is equivalent to a one line cond separated by AND

- **until**: a boolean condition to check in loop context; the loop will continue until the condition is evaluated to false.
- **with_items** or **loop**: the list of element to loop on. If var is not a list, a temporary list containing the variable is used to loop on, so the loop will run only one time.
- **loop_var**: the name of the variable to use for the loop element, by default is it "**item**"

## block or subactions

- **actions**:

- **metrics**:

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
- **ok_status**:
- **auth_config**:
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

### play_script

### set_stats
