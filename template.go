package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	// "strconv"
	"strings"
	ttemplate "text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/peekjef72/httpapi_exporter/encrypt"
)

func convertToBytes(curval any, unit string) (int64, error) {
	var i_value int64
	var err error
	if curval == nil {
		return 0, nil
	}

	// it is a raw value not a template look in "item"
	switch curval := curval.(type) {
	case int:
		i_value = int64(curval)
	case int64:
		i_value = curval
	case float32:
		i_value = int64(curval)
	case float64:
		i_value = int64(curval)
	case string:
		if i_value, err = strconv.ParseInt(strings.Trim(curval, "\r\n "), 10, 64); err != nil {
			i_value = 0
		}
	default:
		i_value = 0
		// value := row[v].(float64)
	}
	switch unit {
	case "kilobyte", "Kb":
		i_value = i_value * 1024
	case "megabyte", "Mb":
		i_value = i_value * 1024 * 1024
	case "gigabyte", "Gb":
		i_value = i_value * 1024 * 1024 * 1024

	}
	return i_value, nil
}

// allow to retrive string header from response's headers
func getHeader(headers http.Header, header string) (string, error) {
	return headers.Get(header), nil
}

// function for template: custom dict hasKey() key that allow to query key from dict of map[any]any type instead of map[string]any
func exporterHasKey(dict any, lookup_key string) (bool, error) {
	res := false

	switch maptype := dict.(type) {
	case map[string]any:
		if _, ok := maptype[lookup_key]; ok {
			res = true
		}
	case map[any]any:
		if _, ok := maptype[lookup_key]; ok {
			res = true
		}
	}

	return res, nil
}

// function for template: custom dict get() key that allow to query key from dict of map[any]any type instead of map[string]any
func exporterGet(dict any, lookup_key string) (any, error) {
	var val any

	switch maptype := dict.(type) {
	case map[string]any:
		if raw_val, ok := maptype[lookup_key]; ok {
			val = raw_val
		}
	case map[any]any:
		if raw_val, ok := maptype[lookup_key]; ok {
			val = raw_val
		}
	default:
		val = ""
	}
	return val, nil
}

// function for template: custom dict set() key with value that allow to set key of dict map[any]any type instead of map[string]any
func exporterSet(dict any, lookup_key string, val any) (any, error) {

	switch maptype := dict.(type) {
	case map[string]any:
		maptype[lookup_key] = val
	case map[any]any:
		maptype[lookup_key] = val
	}

	return dict, nil
}

// function for template: custom dict keys() that allow to obtain keys slide from dict map[any]any type instead of map[string]any
func exporterKeys(dict any) ([]any, error) {
	var res []any

	switch maptype := dict.(type) {
	case map[string]any:
		res = make([]any, len(maptype))
		i := 0
		for raw_key := range maptype {
			res[i] = raw_key
			i++
		}
	case map[any]any:
		res = make([]any, len(maptype))
		i := 0
		for raw_key := range maptype {
			res[i] = raw_key
			i++
		}
	}

	return res, nil
}

// function for template: custom dict values() that allow to obtain values slide from map[any]any type instead of map[string]any
func exporterValues(dict any) ([]any, error) {
	var res []any

	switch maptype := dict.(type) {
	case map[string]any:
		res = make([]any, len(maptype))
		i := 0
		for _, raw_value := range maptype {
			res[i] = raw_value
			i++
		}
	case map[any]any:
		res = make([]any, len(maptype))
		i := 0
		for _, raw_value := range maptype {
			res[i] = raw_value
			i++
		}
	}

	return res, nil
}

// function for template: obtain json marshal representation of obj
func exporterToRawJson(in any) (string, error) {
	var (
		err error
	)
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	switch raw_v := in.(type) {
	case []any:
		buf.WriteString("[")
		llen := len(raw_v)
		for i, raw_v2 := range raw_v {
			err = enc.Encode(&raw_v2)
			if i+1 < llen {
				buf.WriteString(",")
			}
		}
		buf.WriteString("]")
	case map[any]any:
		buf.WriteString("{")
		mlen := len(raw_v)
		i := 0
		for raw_k, raw_v2 := range raw_v {
			str, err2 := exporterToRawJson(raw_k)
			if err2 != nil {
				return "", err2
			}
			buf.WriteString(str)
			buf.WriteString(":")
			str, err2 = exporterToRawJson(raw_v2)
			if err2 != nil {
				return "", err2
			}
			buf.WriteString(str)
			i++
			if i < mlen {
				buf.WriteString(",")
			}
		}
		buf.WriteString("}")
	// case string:
	default:
		err = enc.Encode(&raw_v)
	}

	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil

}

// function to decrypt password from shared key sent by caller
func exporterDecryptPass(passwd string, auth_key string) (string, error) {
	if strings.Contains(passwd, "/encrypted/") {
		ciphertext := passwd[len("/encrypted/"):]
		cipher, err := encrypt.NewAESCipher(auth_key)
		if err != nil {
			err := fmt.Errorf("can't obtain cipher to decrypt: %s", err)
			// level.Error(c.logger).Log("errmsg", err)
			return passwd, err
		}
		passwd, err = cipher.Decrypt(ciphertext, true)
		if err != nil {
			err := fmt.Errorf("invalid key provided to decrypt: %s", err)
			// level.Error(c.logger).Log("errmsg", err)
			return passwd, err
		}
	}

	return passwd, nil
}
func mymap() ttemplate.FuncMap {
	sprig_map := sprig.FuncMap()
	// my_map := make(map[string]interface{}, len(sprig_map)+1)
	// for k, v := range sprig_map {
	// 	my_map[k] = v
	// }
	// my_map["convertToBytes"] = convertToBytes
	sprig_map["convertToBytes"] = convertToBytes
	sprig_map["getHeader"] = getHeader
	sprig_map["exporterDecryptPass"] = exporterDecryptPass
	sprig_map["exporterHasKey"] = exporterHasKey
	sprig_map["exporterGet"] = exporterGet
	sprig_map["exporterSet"] = exporterSet
	sprig_map["exporterKeys"] = exporterKeys
	sprig_map["exporterValues"] = exporterValues
	sprig_map["exporterToRawJson"] = exporterToRawJson

	return sprig_map
}