package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"strings"
	ttemplate "text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/peekjef72/passwd_encrypt/encrypt"
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

func convertBoolToInt(curval any) (int64, error) {
	var i_value int64

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
		s_value := strings.ToLower(curval)
		switch s_value {
		case "true":
			i_value = 1
		case "yes":
			i_value = 1
		case "ok":
			i_value = 1
		default:
			i_value = 0
		}
	case map[string]any:
		i_value = int64(len(curval))
	case map[any]any:
		i_value = int64(len(curval))
	case []any:
		i_value = int64(len(curval))
	case []string:
		i_value = int64(len(curval))
	default:
		i_value = 0
	}
	return i_value, nil
}

// allow to retrive string header from response's headers
func getHeader(headers http.Header, header string) (string, error) {
	return headers.Get(header), nil
}

// allow to retrive string header from response's headers
func getCookie(cookies []*http.Cookie, find_cookie string) (string, error) {
	found_cookie := ""

	for _, cookie := range cookies {
		if cookie.Name == find_cookie {
			if err := cookie.Valid(); err != nil {
				return "", err
			}
			found_cookie = cookie.Value
			break
		}
	}
	return found_cookie, nil
}

func QueryEscape(s string) string {
	return url.QueryEscape(s)
}

func exists(data any) (bool, error) {
	res := false
	if data != nil {
		return true, nil
	}
	return res, nil
}

func getfloat(val any) (float64, bool) {
	if val == nil {
		return 0, true
	}
	var (
		f_value float64
		err     error
	)

	// it is a raw value not a template look in "item"
	switch curval := val.(type) {
	case int:
		f_value = float64(curval)
	case int64:
		f_value = float64(curval)
	case float64:
		f_value = curval
	case string:
		if f_value, err = strconv.ParseFloat(strings.Trim(curval, "\r\n "), 64); err != nil {
			f_value = 0
		}
	default:
		f_value = 0
	}
	return f_value, false
}

const (
	opEqual uint = iota
	// opNE
	// opLT
	// opLE
	opGT
	opGE
)

func checkOp(op uint, val1 any, val2 any) bool {
	res := false
	f1, isnil1 := getfloat(val1)
	f2, isnil2 := getfloat(val2)

	if isnil1 || isnil2 {
		switch op {
		case opEqual:
			res = (isnil1 == isnil2)
		case opGT:
			if isnil1 || !isnil2 {
				res = true
			}
		case opGE:
			if isnil1 && isnil2 {
				res = true
			}
		}
	} else {
		switch op {
		case opEqual:
			res = (f1 == f2)
		case opGT:
			if f1 > f2 {
				res = true
			}
		case opGE:
			if f1 >= f2 {
				res = true
			}
		}
	}
	return res
}

func exporterEQ(val1 any, val2 any) bool {
	return checkOp(opEqual, val1, val2)
}

// not equal is reverse of eq
func exporterNE(val1 any, val2 any) bool {
	return !checkOp(opEqual, val1, val2)
}
func exporterGE(val1 any, val2 any) bool {
	return checkOp(opGE, val1, val2)
}
func exporterGT(val1 any, val2 any) bool {
	return checkOp(opGT, val1, val2)
}

// less equal than (val1 <= val2) <=> val2 >= val1: reverse ope to greater equal than
func exporterLE(val1 any, val2 any) bool {
	return checkOp(opGE, val2, val1)
}

// less than (val1 < val2) <=> val2 > val1: reverse ope to greater than
func exporterLT(val1 any, val2 any) bool {
	return checkOp(opGT, val2, val1)
}

func exporterLEN(dict any) int64 {
	var res int = 0

	switch maptype := dict.(type) {
	case map[string]any:
		res = len(maptype)
	case map[any]any:
		res = len(maptype)
	case []any:
		res = len(maptype)
	case []string:
		res = len(maptype)
	case string:
		res = len(maptype)
	}

	return int64(res)
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
	case map[string]string:
		if raw_val, ok := maptype[lookup_key]; ok {
			val = raw_val
		}
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
			return passwd, err
		}
		passwd, err = cipher.Decrypt(ciphertext, true)
		if err != nil {
			err := fmt.Errorf("invalid key provided to decrypt: %s", err)
			return passwd, err
		}
	}

	return passwd, nil
}

func exportLookupAddr(ip string) (string, error) {
	host := "<no reverse host>"
	res, err := net.LookupAddr(ip)
	if err != nil {
		return host, nil
	}
	if len(res) > 0 {
		host = res[0]
		return strings.TrimSuffix(host, "."), nil
	}
	return host, nil
}

func exporterRegexExtract(regex_pat string, search_str string) ([]string, error) {

	if re, err := regexp.Compile(regex_pat); err != nil {
		return nil, fmt.Errorf(`invalid regexp expression '%s': %s`, regex_pat, err.Error())
	} else {
		return re.FindStringSubmatch(search_str), nil
	}
}

func mymap() ttemplate.FuncMap {
	sprig_map := sprig.FuncMap()
	sprig_map["convertToBytes"] = convertToBytes
	sprig_map["convertBoolToInt"] = convertBoolToInt
	sprig_map["getHeader"] = getHeader
	sprig_map["getCookie"] = getCookie
	sprig_map["queryEscape"] = QueryEscape

	sprig_map["exists"] = exists
	sprig_map["EQ"] = exporterEQ
	sprig_map["NE"] = exporterNE
	sprig_map["GE"] = exporterGE
	sprig_map["GT"] = exporterGT
	sprig_map["LE"] = exporterLE
	sprig_map["LT"] = exporterLT
	sprig_map["LEN"] = exporterLEN
	sprig_map["exporterDecryptPass"] = exporterDecryptPass
	sprig_map["exporterHasKey"] = exporterHasKey
	sprig_map["exporterGet"] = exporterGet
	sprig_map["exporterSet"] = exporterSet
	sprig_map["exporterKeys"] = exporterKeys
	sprig_map["exporterValues"] = exporterValues
	sprig_map["exporterToRawJson"] = exporterToRawJson
	sprig_map["lookupAddr"] = exportLookupAddr
	sprig_map["exporterRegexExtract"] = exporterRegexExtract

	return sprig_map
}
