# Tutorial: Building an httpapi_exporter Configuration from curl for the IBM FlashSystem API

> **Objective**: Understand how to create a complete configuration for httpapi_exporter using IBM FlashSystem API exploration with `curl`.
> This tutorial covers authentication, metric collection, sample file usage, and the file reader mode for development.

---

## Introduction

This guide walks you through a **step-by-step method** to build an httpapi_exporter configuration for the IBM FlashSystem REST API, starting from scratch using only `curl` and IBM's documentation.

You will learn how to:

- Explore the IBM FlashSystem API using `curl` (endpoints, headers, auth, formats)
- Build an authentication profile (`profile.yml`)
- Configure collectors for system metrics
- Create targets to reach your devices
- Use sample files for development and testing
- Test your configuration using the file reader mode (no real system required)

---

## Prerequisites

- Access to an IBM FlashSystem (or a test system for development)
- `curl` installed
- A user account with monitoring privileges on the IBM FlashSystem
- Authentication credentials (username and password)
- httpapi_exporter installed (for final testing)

---

## Method Summary

The complete method involves **4 steps**:

1. **Discovery**: Explore the API using `curl` to understand endpoints and functionality
2. **Configuration**: Translate `curl` results → httpapi_exporter scripts (`query`, `set_fact`, `metrics`)
3. **Development**: Use sample files with the file reader mode to test without a real system
4. **Deployment**: Configure Prometheus and start the exporter with your final configuration

---

# Step 1: Explore the IBM FlashSystem API using curl

This phase is **critical**: you cannot properly configure httpapi_exporter without knowing the target API.

---

## 1.1. Discovering endpoints and general format

```bash
# Request available API versions
curl -k -v https://<HOST_IP>:7443/rest/v1
```

**Example expected response** (HTTP 200):

```
< HTTP/1.1 200 OK
< Content-Type: application/json
{
    "versions": ["2.0"],
    "swagger": "https://<HOST_IP>:7443/rest/v1/swagger.json"
}
```

**What this teaches you**:

- ✅ All endpoints begin with `/rest/v1/`
- ✅ The system uses HTTPS on port 7443
- ✅ Responses are in JSON format
- ✅ Entry point to retrieve Swagger documentation

---

## 1.2. Discovering required authentication

IBM FlashSystem uses token-based authentication. Let's test:

```bash
# Basic test without authentication
curl -k -v https://<HOST_IP>:7443/rest/v1/lscurrentuser \
  -X POST \
  -H "Content-Type: application/json"
```

**Possible responses**:

- **HTTP 401 or 403** ➜ Authentication required ➜ Move to next section
- **HTTP 200** ➜ Endpoint is accessible without auth (rare for FlashSystem)
- **HTTP 500/404** ➜ Incorrect endpoint or system

---

## 1.3. Authenticating to the API

IBM FlashSystem uses a JWT token system.

**Steps using curl**:

1. **Log in and obtain a token**:

```bash
curl -k -v https://<HOST_IP>:7443/rest/v1/auth \
  -X POST \
  -H "Content-Type: application/json" \
  -H "X-Auth-Username: supervision" \
  -H "X-Auth-Password: Your_Password"
```

2. **Check the response**:

```json
HTTP/1.1 200 OK
{
    "token": "eyJ0eXAiOiJKV1QiLCJhbGci...")
}
```

**What this teaches you**:

- Authentication endpoint: `/auth`
- Method: POST
- Required headers: `Content-Type: application/json` and `X-Auth-Username`, `X-Auth-Password`
- Server returns a **token** in a JSON object
- This token must be used in subsequent requests in the `X-Auth-Token` header
- Errors: **401** (invalid credentials) or **403** (access denied)


---

## 1.4. First request with the authentication token

```bash
# Ping endpoint: /lscurrentuser (checks if we're logged in)
curl -k -v https://<HOST_IP>:7443/rest/v1/lscurrentuser \
  -X POST \
  -H "Content-Type: application/json" \
  -H "X-Auth-Token: eyJ0eXAiOiJKV1QiLCJhbGci..."
```

**Expected response** (HTTP 200):

```json
{
    [
        {"name": "supervision"},
        {"role": "Monitor"},
        ...
    ]
}
```

**Why this is important**:

- This endpoint will serve as a **ping** for httpapi_exporter
- It confirms that authentication works
- It returns data about the logged-in user

---

## 1.5. Collecting metrics and other useful endpoints

IBM FlashSystem exposes many endpoints depending on hardware and software configuration.

**Example: System metrics** (/lssystemstats):

```bash
curl -k -v https://<HOST_IP>:7443/rest/v1/lssystemstats \
  -X POST \
  -H "Content-Type: application/json" \
  -H "X-Auth-Token: eyJ0eXAiOiJKV1QiLCJhbGci...
```

**Example response** (excerpt):

```json
[
    {
        "stat_name": "cpu_pc",
        "stat_current": "20",
        "stat_peak": "24",
        "stat_peak_time": "260708152038"
    },
    {
        "stat_name": "fc_mb",
        "stat_current": "500",
        "stat_peak": "750",
        "stat_peak_time": "..."
    },
    ... 300+ additional metrics
]
```

**Analysis**:

- This is a **JSON array** of system metrics
- Each object has: `stat_name` (name), `stat_current` (current value), `stat_peak` (peak), `stat_peak_time` (timestamp)
- Regular format for all metrics

**Other common endpoints**:

- `/lsenclosures` : Enclosure information
- `/lsdrive` : Physical disk information
- `/lsportfc` : Fibre Channel ports
- `/lsportethernet` : Ethernet ports

**Tips**:

- Save some responses in files (e.g. `ibmfs_systemstats.json`) for future reference
- Note the exact names of metrics: `cpu_pc`, `fc_mb`, `vdisk_io`, etc.

---

## 1.6. Analyzing differences between `/lssystemstats` and other endpoints

| Endpoint       | Response Format       | Structure                   |
|----------------|---------------------|-----------------------------|
| `/lssystemstats`| JSON Array          | `[{"stat_name": "...", "stat_current": "..."}, ...]` |
| `/lsenclosures`| JSON Object         | `{"id": "123", "status": "online", ...}` |
| `/lscurrentuser`| JSON Array          | `[{"name": "..."}, {"role": "..."}]` |

>> **Understanding this format is essential** to correctly write httpapi_exporter scripts.

---

# Step 2: Building the httpapi_exporter configuration

Now that you know the API, you'll build the configuration with 3 parts: **profile**, **collectors**, and **target**.
This configuration is inspired by the official documentation but explained step by step.

---

## 2.1. Recommended file structure

```bash
/etc/httpapi_exporter/
├── ibm_flashsystem/
│   ├── profiles/
│   ├── samples/                              # Test JSON files
│   │   ├── ibmfs_auth.json
│   │   ├── ibmfs_systemstats_metrics.json
│   │   ├── ...
│   │   ├── funcs/ibmfs.js                    # Utility JavaScript functions
│   │   └── ibmfs_profile.yml                 # Authentication and ping scripts
│   ├── metrics/
│   │   ├── ibmfs_systemstats.collector.yml   # System metrics
│   │   ├── ibmfs_enclosure.collector.yml     # Enclosure metrics
│   │    └── ibmfs_drive.collector.yml        # Drive metrics
│   └── targets/
│       └── flashsystem-1.yml                 # Specific target configuration
├── ...
```

---

## 2.2. Writing the IBM FlashSystem profile: /etc/httpapi_exporter/ibm_flashsystem/profiles/ibmfs_profile.yml

**Complete file with detailed explanations**:

```yaml
---
# Profile for IBM FlashSystem
ibmfs:
  # Node.js modules for utility functions
  modules:
    # ./ is required for local paths
    ibmfs: '$CONFIG_PATH/profiles/funcs/ibmfs.js'
  
  # Prefix for all metrics generated from this profile
  metric_prefix: ibmfs
  
  # Configuration scripts
  scripts:
    # --- SCRIPT INIT: Initial configuration of headers and default parameters ---
    init:
      - name: default headers configuration
        set_fact:
          # Default HTTP headers to apply to all requests
          headers:
            - name: Content-type
              value: "application/json"
          # Default network parameters
          scheme: https
          port: 7443
          verifySSL: false
          base_url: /rest/v1
          # Number of connection attempts for authentication
          queryRetry: 3
     
    # --- SCRIPT AUTH_CHECK: Post-login check and token management ---
    auth_check:
      - name: Analyze successful login response
        set_fact:
          # Add the token returned by /auth in the X-Auth-Token header
          headers:
            - name: "X-Auth-token"
              value: $auth.token
          # Mark as logged in for the rest of the exporter's queries
          logged: true
          # Remaining login attempts
          login_retry: $queryRetry
        when:
          - 'js: status_code == 200 && auth !== undefined && auth.token != undefined'
     
      - name: Analyze unauthorized (401/403) response
        set_fact:
          logged: false
          login_retry: $queryRetry
          # Remove token from header on auth error
          headers:
            - name: "X-Auth-token"
              action: delete
        when:
          - 'js: (status_code == 401) || (status_code == 403)'
     
      - name: Analyze unexpected error response
        set_fact:
          logged: false
          # Increment login attempt counter
          login_retry: 'js: ++login_retry'
        when:
          - 'js: (status_code != 200) && (status_code != 401) && (status_code != 403)'
   
    # --- SCRIPT LOGIN: Login to the IBM FlashSystem API ---
    login:
      - name: Initialize login retry loop
        vars:
          # Initialize attempt counters
          login_retry: 0
          status_code: 0
        # Loop while login_retry < queryRetry
        until: 'js: login_retry < queryRetry'
        
        actions:
          # Phase 1: Send credentials to obtain a token
          - name: login phase
            vars:
              headers:
                - name: 'X-Auth-Username'
                  value: supervision
                - name: 'X-Auth-Password'
                  # Note: password encrypted or plaintext depending on configuration (see below)
                  value: "js: exporter.decryptPass( password, auth_key )"
            query:
              url: /auth
              method: post
              ok_status: 200
              # Don't store raw response; just extract the token
              var_name: auth
              parser: json # Default parser
             
          # Phase 2: Check login response
          - name: analyze login response
            play_script: auth_check
   
    # --- SCRIPT PING: Availability check (called before all collections) ---
    ping:
      - name: Check connection to IBM FlashSystem
        query:
          # Endpoint to be checked: /lscurrentuser
          url: /lscurrentuser
          method: post
          ok_status: 200
          # Store response in '_' (not used in this case)
          var_name: _
```

**Key points to remember**:

- ✅ **`init`**: Configures default headers (Content-Type) and network parameters
- ✅ **`login`**: Performs the `/auth` request with username/password to obtain a token
- ✅ **`auth_check`**: Verifies login success/failure and manages the token in headers
- ✅ **`ping`**: Checks if the system is accessible and authenticated (200 status expected)
- ✅ **`queryRetry`**: Number of attempts before authentication failure (3 by default)
- ⚠️ **`CONFIG_PATH`**: Environment variable must be set at startup to directory where the config stands (`/etc/httpapi_exporter/ibm_flashsystem`)
- ⚠️ **`verifySSL: false`**: Disables SSL verification (use only in test environments or with legitimate self-signed certificates. **Remove in production** and use valid certificates.)

---

## 2.3. Encrypting the password in the configuration

**IMPORTANT**: Never store passwords in plaintext in configuration files!

**Possible methods**:

1. **Built-in encryption** (recommended):

```bash
# Encrypt password
/path/to/httpapi_exporter/passwd_encrypt
```

Use the tool's guidance to generate an encrypted password string.

1. **Environment variables**:

Set the variable in exporter startup env:

```bash
IBMFS_EXPORTER_USER=\<login\> IBMFS_EXPORTER_PASSWD=\<password>
```

and use it in target definition or in global auth definition.

```yaml
...
targets:
  # default target is used as a pattern for exporter queries with target name not defined locally.
  - name: default
    scheme: https
    host: template
    port: 7443
    auth_config:
      # mode: basic|token|[anything else:=> user defined login script]
      mode: script
      user: $env:IBMFS_EXPORTER_USER
      password: $env:IBMFS_EXPORTER_PASSWD

---
```

## 2.4. Writing a metrics collector: /etc/httpapi_exporter/ibm_flashsystem/metrics/ibmfs_systemstats.collector.yml

**Based on the `/lssystemstats` endpoint** whose structure you examined with `curl`.

**Complete file**:

```yaml
---
# Collector declaration
collector_name: ibmfs_systemstats_metrics
# Metric prefix for generated metrics (e.g. ibmfs_systemstats_cpu_pc)
metric_prefix: ibmfs_systemstats

# Collection scripts
scripts:
  # Script 1: Retrieve system data
  get_lssystemstats:
    - name: Query system statistics endpoint
      query:
        url: /lssystemstats
        method: post
        # Store JSON array result in 'results' variable
        var_name: results
        parser: json
     
    # Script 2: Prepare metrics from raw data
    - name: Format metrics from raw data
      set_fact:
        metrics: >
          js:
            # Initialize metrics array
            var metrics = []
            
            # Loop through each metric returned by the API
            for( var item of results ) {
              # Package: each metric has a name, value, help, and type
              var metric = {
                name: item.stat_name,
                value: item.stat_current,
                help: "",
                type: "gauge"
              }
              
              # Map stat_name → complete Prometheus metric
              # Based on IBM FlashSystem documentation
              switch (item.stat_name) {
                # ----------------- CPU -----------------
                case "compression_cpu_pc":
                  metric.help = "Percentage of CPU used for compression";
                  break;
                case "cpu_pc":
                  metric.help = "Percentage of allocated CPU capacity used by the system";
                  break;
                
                # ----------------- FC Traffic -----------------
                case "fc_mb":
                  metric.help = "Total FC megabytes transferred per second (MBps) for all traffic";
                  metric.type = "counter";
                  break;
                case "fc_io":
                  metric.help = "Total FC input/output operations per second";
                  metric.type = "counter";
                  break;
                
                # ----------------- SAS Traffic -----------------
                case "sas_mb":
                  metric.help = "Total SAS megabytes transferred per second (MBps)";
                  metric.type = "counter";
                  break;
                
                # ----------------- Cache -----------------
                case "write_cache_pc":
                  metric.help = "Percentage of write cache usage";
                  break;
                case "total_cache_pc":
                  metric.help = "Total percentage of both write and read cache usage";
                  break;
                
                # ----------------- Volumes -----------------
                case "vdisk_mb":
                  metric.help = "Average megabytes transferred per second (MBps) for volumes";
                  metric.type = "counter";
                  break;
                case "vdisk_io":
                  metric.help = "Average I/O operations per second for volumes";
                  break;
                case "vdisk_ms":
                  metric.help = "Average response time in milliseconds for volume requests";
                  break;
                
                # ... (continue for all metrics, see real file for full mapping)
                default:
                  # Skip unknown or undocumented metrics
                  continue;
              }
              
              # Add metric to the array
              metrics.push(metric);
            }
            metrics
    
    # Script 3: Generate final Prometheus metrics
    - name: Output generated metrics
      loop: $metrics
      when:
        - 'js: metrics.length > 0'
      metrics:
        - metric_name: $name
          help: $help
          type: $type
          values:
            _ : $value
```

**Methodical explanations**:

1. **`collector_name`**: Unique collector name (for `collectors:` in target)
2. **`metric_prefix`**: Prefix for **all** metrics generated (e.g. `ibmfs_systemstats_cpu_pc`)
3. **`query`**: HTTP request calling `/lssystemstats` (POST, headers already configured by profile)
4. **`parser: json`**: The `json` parser decodes the HTTP response to JavaScript object → variable `results`
5. **JavaScript transformation**:
   - Loop through each element in `results` (each API metric row)
   - **Switch/case**: Converts `stat_name` to complete Prometheus metrics with names, help, types
   - The result is an array of `metrics` in Prometheus format
6. **Output**: The `metrics:` action generates Prometheus metrics as-is

**Why this format?**

- The `stat_current` values range from 0 to 100 for percentages → **gauge**
- The `*_mb`, `*_io` count operations → **counter**
- The `*_ms` measure time → **gauge** or **histogram**

>> **Tip**: Always check IBM FlashSystem documentation for the type of each metric.

---

## 2.5. Writing the target for your specific device: /etc/httpapi_exporter/ibm_flashsystem/targets/flashsystem-1.yml

**Configuration file to target a specific IBM FlashSystem**:

```yaml
# Unique name of the target (used in Prometheus scraping)
name: flashsystem-1-ntt01a

# Network configuration
scheme: https
host: ibm-ntt01a.example.com  # IP or DNS of your IBM FlashSystem
port: 7443

# Security parameters (adjust according to your environment)
verifySSL: false
# In production: configure with valid certificates and set to true

# Device-specific variables (if not in profile globals)
auth_config:
  # mode: basic|token|[anything else:=> user defined login script]
  mode: script
  user: my_specific_user_for_that_target
  password: "/encrypted/specif_encrypted_password"

# Prometheus labels: metadata added to all metrics
labels:
  - name: vendor
    value: ibm
  - name: location
    value: ntt
  - name: environment
    value: production

# Reference to profile to use
profile: ibmfs

# List of collectors to execute for this target
# - ~ ibmfs_system* : All collectors starting with 'ibmfs_system'
# - ibmfs_drive     : Specific collector for drive metrics
# The '~' syntax means 'matches pattern', '!~' means 'does not match'
collectors:
  - ~ ibmfs_system*
  - ibmfs_drive
  - ibmfs_enclosure
```

**Key points**:

- ✅ **`profile: ibmfs`**: Uses the `ibmfs_profile.yml` profile created earlier
- ✅ **`collectors`**: List of collectors to execute (.collector.yml files)
- ✅ **`customs`**: Specific variables (to decrypt the password)
- ✅ **`labels`**: Prometheus metadata for filtering and grouping
- ✅ **`scheme/host/port`**: Exact network coordinates of your device

---

## 2.6. Summary of the 3 critical files

| File | Purpose | Type |
|------|---------|------|
| `ibm_flashsystem/profiles/ibmfs_profile.yml` | Authentication, ping, tokens | Profile YAML |
| `ibm_flashsystem/metrics/ibmfs_*.collector.yml` | Data collection and transformation API → Prometheus | Collector YAML |
| `ibm_flashsystem/targets/flashsystem-1.yml` | Specific target configuration with host, password, labels and collector choice | Target YAML |

---

# Step 3: Development and testing using sample files (file reader mode)

You have now configured:

- **Profile** for IBM FlashSystem authentication
- **Collectors** for system metrics
- **Target** for your device

**BUT**: To test your configuration without a real system, use the **file reader** mode with the sample files.

---

## 3.1. Understanding file reader mode

The `ibmfs_filereader` profile is a **variant of the `ibmfs` profile** that uses static JSON files instead of making real HTTP requests.


**When to use it**:

- Developing new collectors
- Client demonstrations
- Testing in a CI/CD environment
- Troubleshooting without system access

**How it works**:

- Instead of querying `/auth` via HTTP, the profile reads `ibm_flashsystem/profiles/samples/ibmfs_auth.json`
- Instead of querying `/lssystemstats` via HTTP, the profile reads `ibm_flashsystem/profiles/samples/ibmfs_systemstats_metrics.json`
- **Collectors don't need to change**: They use the same variables (`results`, `auth`) but populated from files instead of requests.

---

## 3.2. Using the existing `ibmfs_filereader` profile

The file `ibm_flashsystem/etc/ibmfs/profiles/ibmfs_filereader_profile.yml` is already configured for this.

**Directory structure**:

```
ibm_flashsystem/profiles/
├── ibmfs_filereader_profile.yml
└── samples/
    ├── ibmfs_auth.json                              # Simulates /auth
    ├── ibmfs_lscurrentuser.json                     # Simulates /lscurrentuser
    ├── ibmfs_systemstats_metrics.json               # Simulates /lssystemstats
    ├── ibmfs_enclosure_metrics.json                 # Simulates /lsenclosures
    ├── ibmfs_enclosurecanister_metrics.json         # ...
    ├── ibmfs_portfc_metrics.json                    # ...
    └── ... (15 other sample files)
```

**How it works**

The `ping` script in `ibmfs_filereader_profile.yml` contains:

```yaml
ping:
  - name: read from files and simulate API responses
    set_fact:
      query_status: 200
      logged: true
      alternates: >-
        js:
          # Get CONFIG_PATH to access samples
          var config_path = process.env("CONFIG_PATH")
          config_path = config_path == undefined || config_path == '' ? '.': config_path;
          
          # List of sample files to read (corresponds to IBM FlashSystem endpoints)
          var elements = [
            { name: 'lscurrentuser', file: 'ibmfs_lscurrentuser.json' },
            { name: 'auth', file: 'ibmfs_auth.json' },
            { name: 'lsenclosure', file: 'ibmfs_enclosure_metrics.json' },
            { name: 'lssystemstats', file: 'ibmfs_systemstats_metrics.json' },
            { name: 'lsdrive', file: 'ibmfs_drive_metrics.json' },
            // ... 12 other endpoints
          ];
          
          var res = {};
          for (var element of elements) {
            # Read sample file using Node.js fs module
            var buf = fs.readFileSync(config_path + '/profiles/samples/' + element.file)
            if( buf === undefined ) {
              console.error("Can't read file", element.file);
            }
            else {
              res[element.name] = buf;
            }
          }
          res
```

> ⚠️ **IMPORTANT**: The JavaScript code uses the `fs` module to read static files. This is why the `fs` module must also be included in the list of required modules as shown in the profile.

---

## 3.3. How to run httpapi_exporter in file reader mode

**Command**:

```bash
/path/to/httpapi_exporter \
  --config.file=config.yml                #[Your global config] \
  --test-mode ibmfs_filereader
```

**What the configuration does**:

1. The default profile `ibmfs` used for all targets is replaced by the profile `ibmfs_filereader` defined in `ibmfs_filereader_profile.yml`
2. The exporter uses the sample files instead of real HTTP requests for `query` actions with an `alternate` defined and referencing an existing key in the associative array (dict) `alternates` defined in the `ibmfs_filereader` profile
3. Collectors generate metrics as if the API were real
4. **Result**: You get Prometheus metrics without needing access to an IBM FlashSystem!


> ✅ **Collectors remain identical**: `ibmfs_systemstats.collector.yml` etc.

---

## 3.4. Example of a sample file to understand structure

Let's look at `ibm_flashsystem/profiles/samples/ibmfs_systemstats_metrics.json` (excerpt):

```json
[
    {"stat_name": "compression_cpu_pc", "stat_current": "0"},
    {"stat_name": "cpu_pc", "stat_current": "20"},
    {"stat_name": "fc_mb", "stat_current": "8517"},
    {"stat_name": "fc_io", "stat_current": "196698"},
    {"stat_name": "sas_mb", "stat_current": "0"},
    ... 317 other metrics (see full file)
]
```

**What this shows you**:

- 320+ IBM FlashSystem system metrics
- Each metric has: `stat_name` and `stat_current` (current value)
- Exactly the same format as the real API → **Verify your collectors match**

**Another example**: `ibm_flashsystem/profiles/samples/ibmfs_auth.json`

```json
{
    "token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9..."
}
```

-> Used by the profile `login` to simulate a successful authentication.

---

## 3.5. Creating a new sample file to develop a new collector

**Scenario**: You want to develop a collector for a new endpoint `/lsportfc` and simulate data.

**Steps**:

1. **Capture the real response**:

```bash
curl -k -v https://<HOST>:7443/rest/v1/lsportfc \
  -X POST \
  -H "X-Auth-Token: token"
```

1. **Save to** `ibm_flashsystem/profiles/samples/ibmfs_portfc_metrics.json`
1. **Add to** `ibm_flashsystem/profiles/ibmfs_filereader_profile.yml`:

```yaml

var elements = [
  ...,
  { name: 'lsportfc', file: 'ibmfs_portfc_metrics.json' },
];
```

1. **Write a collector** `ibm_flashsystem/metrics/ibmfs_portfc.collector.yml` with the mapping `stat_name → Prometheus`
1. **Test with** httpapi_exporter in file reader mode

>> **Tip**: Sample files are the **living documentation** of the API. They allow you to see exactly what data is returned.

---

# Step 4: Final deployment and Prometheus integration

All configuration steps are complete. It remains to integrate httpapi_exporter into your monitoring ecosystem.

---

## 4.1. Complete configuration in config.yml global

Example minimal (adjust according to your real file):

```yaml
# Global defaults.
global:
  # max timeout for the exporter: if prometheus sends a value greater than scrape_timeout, scrape_timeout will be used
  scrape_timeout: 30s
  # Subtracted from Prometheus' scrape_timeout to give us some headroom and prevent Prometheus from timing out first.
  scrape_timeout_offset: 500ms
  # Minimum interval between collector runs: by default (0s) collectors are executed on every scrape.
  min_interval: 0s
  # all unsuccessful queries will be retried this number of times
  query_retry: 3
  # all metrics will be named "[metric_prefix]_[metric_name]"
  metric_prefix: "ibm_flashsystem"
  # all http codes that will consider the connection has an invalid auth and must do a Login()
  invalid_auth_code: [401,403]
  exporter_name: ibm_flashsystem

profiles_file_config:
  - "profiles/*_profile.yml"

# list of authentication configuration to use to access to a target.
auth_configs:
  prometheus_encrypted:
    mode: script
    user: prometheus
    password: "/encrypted/__________XXXXXXX_______________XXXXXXX_______"

# The targets to monitor and the collectors to execute on it.
targets:
  # default target is used as a pattern for exporter queries with target name not defined locally.
  - name: default
    scheme: https
    host: template
    port: 7443
    auth_name: prometheus_encrypted
    # auth_config:
    #   # mode: basic|token|[anything else:=> user defined login script]
    #   mode: script
    #   user: login
    #   password: "/encrypted/base64_encrypted_password_by_passwd_crypt_cmd"
    profile: ibmfs
    collectors:
      - ~ ibmfs_.*_metrics

  - targets_files: [ "targets/*.yml" ]

# Collector files specifies a list of globs. One collector definition is read from each matching file.
collector_files:
  - "metrics/*.collector.yml"

```

---

## 4.2. Prometheus configuration: prometheus.yml

Add your httpapi_exporter target:

```yaml
scrape_configs:
  - job_name: 'ibm_flashsystem'
    static_configs:
      - targets: ['localhost:9091']  # or the exporter's host/IP
```

---

## 4.3. Starting httpapi_exporter

```bash
# In development with file reader
CONFIG_PATH=/path/to/ibm_flashsystem /path/to/httpapi_exporter \
  --log.level debug \
  --web.listen-address 0.0.0.0:9300 \
  --config.file  /path/to/ibm_flashsystem/config.yml \
  --test-mode ibmfs_filereader

# In production with real system
CONFIG_PATH=/path/to/ibm_flashsystem /path/to/httpapi_exporter \
  --config.file=/home/config.yml
```

or as a service with systemctl

---

## 4.4. Verifying generated metrics

```bash
# Connect to httpapi_exporter
curl "http://localhost:9091/metrics?target=<target_name>[&auth_key=<key_to_decipher_encryted_password>]
```

**Check**:

- The metric `ibmfs_up 1` must be present and set to 1 → system accessible
- All your system metrics must be generated with the format: `ibmfs_systemstats_cpu_pc{...} 20`
- Labels must be correct (environment="production", vendor="ibm")

**Example output**:

```text
# HELP ibmfs_systemstats_cpu_pc Percentage of CPU used by the system
# TYPE ibmfs_systemstats_cpu_pc gauge
ibmfs_systemstats_cpu_pc{environment="production",vendor="ibm"} 20

# HELP ibmfs_systemstats_fc_mb Total FC megabytes per second
# TYPE ibmfs_systemstats_fc_mb counter
ibmfs_systemstats_fc_mb{environment="production",vendor="ibm"} 8517
```

---

## 4.5. Final integration into your monitoring stack

- Configure Grafana to visualize IBM FlashSystem metrics
- Add specific dashboards (CPU, I/O, cache, etc.)
- Set up Prometheus alerts on critical metrics (`ibmfs_up 0`, `cpu_pc > 95`, etc.)

> 📌 **Note**: Check the Prometheus/Grafana community for ready-to-use IBM FlashSystem dashboards.

---

# Summary of the complete method

## 5.1. Complete checklist

1. ✅ **Discovery**: Explore `/rest/v1` and `/auth`, `/lscurrentuser`, `/lssystemstats` using `curl`
2. ✅ **Profile**: Create `ibmfs_profile.yml` with `init`, `login`, `auth_check`, `ping`
3. ✅ **Collectors**: Create `ibmfs_*.collector.yml` with mapping `stat_name → Prometheus`
4. ✅ **Target**: Create `targets/flashsystem-1.yml` with host, encrypted password, labels and choice of collectors
5. ✅ **Development**: Use `samples/` with `ibmfs_filereader_profile.yml` to test without system access
6. ✅ **Deployment**: Configure Prometheus and httpapi_exporter, verify metrics

---

## 5.2. Example work sequence

1. A developer discovers the IBM FlashSystem API using `curl`
2. They create a profile (`ibmfs_profile.yml`) and collectors (`sysstats.collector.yml`)
3. They use sample files (`samples/ibmfs_systemstats_metrics.json`) to validate collectors without a real system
4. Once validated, they create a target (`targets/ntt-flash.yml`) for the real device
5. They deploy httpapi_exporter with (config.yml) and Prometheus (prometheus.yml)
6. Metrics appear in Prometheus/Grafana!

---

# Annexes: Tools and references

## Annex A: httpapi_exporter formats and conventions

| Element | YAML Format | Example |
|---------|------------|---------|
| Profile name | text | `ibmfs:` |
| Scripts | list | `scripts: [login, ping]` |
| Query | action | `url: /auth`, `method: post` |
| JavaScript variable | `js: ...` | `value: "js: $auth.token"` |
| Loop | `loop:` | `loop: $results` |
| Condition | `when:` | `when: status_code == 200` |

---

## Annex B: Variables and password management

### B.1. Accessing variables in scripts

- **Global variables**: Accessible via $variable
- **Loop variables**: $loop_var (e.g. $item)
- **Prometheus variables**: In the target YAML
- **Environment**: Via process.env("VAR") in JS

### B.2. Password encryption

Use the provided cryptographic tool:

```bash
./passwd_encrypt 
give the key: must be 16 24 or 32 bytes long
enter key: 0123456789abcdef 
enter password: mypassword
Encrypting...
Encrypted message hex: CsG1r/o52tjX6zZH+uHHbQx97BaHTnayaGNP0tcTHLGpt5lMesw=
$
```

---

## Annex C: External references and documentation

- **httpapi_exporter GitHub**: https://github.com/peekjef72/httpapi_exporter/
  ➜ Official documentation and examples
- **IBM FlashSystem REST API**: Check your device model or the IBM Redbook
- **Prometheus Metrics**: https://prometheus.io/docs/concepts/metric_types/
  ➜ Understand gauge, counter, histogram, etc.
- **JavaScript in httpapi_exporter**: Uses Node.js → Node.js documentation for used modules and methods
- **Real example**:
  - Profile: `contribs/ibm_flashsystem/etc/ibmfs/profiles/ibmfs_profile.yml`
  - Samples: `samples/` with ibmfs_auth.json, ibmfs_systemstats_metrics.json

---

# Conclusion: You built an httpapi_exporter configuration from start to finish!

This tutorial guided you through an **industrial method** to build an httpapi_exporter configuration for an IBM FlashSystem API:

1. Explore the source API using `curl` to understand endpoints, headers and formats
2. Translate this understanding into httpapi_exporter scripts: profile, collectors and target
3. Use sample files and file reader mode to develop and validate without real hardware
4. Deploy into your monitoring stack (Prometheus/Grafana)

> 📌 **The secret**: "A working `curl` request → A working httpapi_exporter configuration."

>> Share your configurations with the httpapi_exporter community to improve the open-source ecosystem!
