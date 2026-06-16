<!-- cSpell:ignore varmap, svclb, varstring -->
# custom functions for templating

## gotemplate

(**still incomplete**)

Go template is used to manipulate data. So templates inherits from go functions and [sprig](https://masterminds.github.io/sprig/) v3 functions.
Because the exporter uses most of the time data of type "any (interface{})" some of the sprig functions failed.
Here is the list of exporter functions:

name | usage | e.g. |
-- | --- | --- |
exporterDecryptPass | | |
exporterGet [varmap] [keyname] | get the keyname from the map. Like sprig/get function but accepts data of type map[any]any | exporterGet .svclb .svc |
exporterSet | | |
exporterKeys | | |
exporterValues | | |
exporterToRawJson | | |
&nsbp; | | |
lookupAddr [varstring] | obtain hostname from string representing an ip address ; like sprig/getHostByName but for string ip | lookupAddr .node.ipaddress |
convertToBytes value unit | convert the value contained in variable to bytes according to the unit string specified: <ul><li>"kilobyte" or "Kb" multiply by 1024 <li>"megabyte" or "Mb" multiply by 1024 \* 1024<li>"gigabyte" or "Gb"multiply by 1024 \* 1024 \* 1024</ul> | '{{ convertToBytes .result.totalMiB "Mb" }}' |
convertBoolToInt value | convert value that may contain a boolean to 0&#124;1 representation. Value can be of any type. If something is <ul><li>like int or float and different from 0 is 1 else 0<li>string and is lower case 'true' or 'yes' or 'ok' is 1 else 0<li>like map or array and length >0 then 1 or 0</ul> | with {"proc": {"loopCrashing": "true",...}}<br> => '{{ convertBoolToInt .proc.loopCrashing }}<br> => 1' |
getHeader [varmap] | | |
queryEscape [varstring] | | |

LEN [var] | obtain the len of the var. works like sprig/len but accepts data of type any. | |
exporterRegexExtract [regexp var] [search var] : []string | obtain the list of extracted elements from regexp on search string or nil if not found | extract value from line as group 1 of regexp: <br> res: "{{ index  (exporterRegexExtract "^status:\s(.+)$" "status:OK") 1 }}" |

## boolean checks

name | usage | e.g. |
--- | --- | --- |
EQ [var1] [var2] | check equality for 2 variables; accepts any type of data; meaning that the second will be converted to the type of the first | EQ .val "2" |
NE [var1] [var2] | not equal | NE 2 .val |
GE [var1] [var2] | greater equal | |
GT [var1] [var2] | greater than | |
LE [var1] [var2] | less equal | |
LT [var1] [var2] | less than | |
exists [var1] | return boolean if variable exists | exists .config.cluster.node |
exporterHasKey [var] [key] | check if variable is a map and has a key | exporterHasKey .config "cluster" |

## js script/template

As a starting point have a look to apache_exporter [metrics](../contribs/apache/etc/apache/metrics/apache_status.collector.yml)

### functions

Because javascript has a lot of internal functions, anyway a lot more than gotemplate and sprig v3, very few of them has been included in js code.

name | usage | e.g. |
-- | --- | --- |
exporter.convertToBytes(value, unit) | convert the value contained in variable to bytes according to the unit string specified: <ul><li>"kilobyte" or "Kb" multiply by 1024 <li>"megabyte" or "Mb" multiply by 1024 \* 1024<li>"gigabyte" or "Gb"multiply by 1024 \* 1024 * 1024</ul> | 'js: exporter.convertToBytes( 13.45, "Mb" )' |
exporter.convertBoolToInt( value ) | convert value that may contain a boolean to 0&#124;1 representation. Value can be of any type. If something is <ul><li>like int or float and different from 0 is 1 else 0<li>string and is lower case 'true' or 'yes' or 'ok' is 1 else 0<li>like map or array and length >0 then 1 or 0</ul> | with {"proc": {"loopCrashing": "true",...}}<br> => 'js: exporter.convertBoolToInt( proc.loopCrashing)'<br> => 1' |
exporter.default(var) | | |
exporter.decryptPass(varstring) | | |
exporter.getDurationSecond() | | |
exporter.getHeader([varmap] ) | | |
exporter.getCookie( [varmap] ) | | |
exporter.length( Any ) | return length of element; if type is string returns length of string, else if map or slice return number of element in object | 'js: export.length( jobs ) > 0' |
exporter.lookupAddr( hostname string ) | obtain hostname from string representing an ip address ; like sprig/getHostByName but for string ip | 'js: exporter.lookupAddr( node.ipaddress)' |
exporter.queryEscape( url string ) | | |
exporter.getDurationSecond( string ) | | "js: exporter.getDurationSecond( '1d2h30s' )" |
exporter.date(string format, int timestamp) | convert the numeric timestamp into a string using go format | "js: exporter.date("2006-01-02T15:04:05", startTime);" |
exporter.toDate(fmt, string_date) | parse string_date using pattern golang format and return a js Date object | "js: exporter.toDate("2006-01-02T15:04:05Z", item.CreationTimeUTC)" |
exporter.sha1sum( string ) or exporter.sha256sum( string ) or exporter.sha512sum( string ) | compute sha sum of string; useful to obtain unique hash key of string | "js: exporter.sha1sum( 'my_key' )" |

### modules

#### console

Used to log information with level from javascript code to exporter console.
Initialization is not required.

name | usage | e.g. |
-- | --- | --- |
console.log | log with info loglevel; synonym console.info | console.log(...) |
console.error | log with error loglevel | console.error(...) |
console.warn | log with warn loglevel | console.warn(...) |
console.info | log with info loglevel | console.info(...) |
console.debug | log with debug loglevel | console.debug(...) |

#### dns

Used to make dns lookup queries
Initialized by:

```js

 var dns=require('dns')
```

name | usage | e.g. |
-- | --- | --- |
dns.lookup( ... ) | perform a system dns lookup on sent parameter and return a list of object [ {"family": 4\|6, "address": "ip_string"}, ... ] | var entry=dns.lookup( 'www.google.com' ); |

#### fs

used to read local file.

Initialized by:

```js

 var fs=require('fs')
```

name | usage | e.g. |
-- | --- | --- |
fs.readFileSync( ... ) | | var buf=fs.readFileSync( 'myfile' ); |
