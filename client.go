package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log/slog"

	// "net"
	"strconv"
	"strings"

	"sync"
	"time"

	"crypto/tls"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"
	"github.com/peekjef72/passwd_encrypt/encrypt"
	"golang.org/x/exp/slices"
)

var (
	ErrInvalidLogin              = fmt.Errorf("invalid_login")
	ErrInvalidLoginNoCipher      = fmt.Errorf("can't obtain cipher to decrypt")
	ErrInvalidLoginInvalidCipher = fmt.Errorf("invalid key provided to decrypt")
	ErrInvalidQueryResult        = fmt.Errorf("invalid_result_code")
	// context deadline exceeded
	ErrContextDeadLineExceeded = fmt.Errorf("global_scraping_timeout")
)

// type to unmarshal xml stream
const (
	content_node  = iota
	content_value = iota
)

type Content struct {
	Type  int
	Name  string
	Attrs map[string]any
}

//

// Query wraps a sql.Stmt and all the metrics populated from it. It helps extract keys and values from result rows.
type Client struct {
	client *resty.Client

	// logContext []interface{}
	logger *slog.Logger
	sc     map[string]*YAMLScript

	// maybe better to use target symtab with a mutex.lock
	symtab            map[string]any
	invalid_auth_code []int

	// to protect the data during exchange
	content_mutex *sync.Mutex
}

func newClient(target *TargetConfig, sc map[string]*YAMLScript, logger *slog.Logger, gc *GlobalConfig) *Client {

	cl := &Client{
		// logContext:        []interface{}{},
		logger:            logger,
		sc:                sc,
		symtab:            map[string]any{},
		invalid_auth_code: gc.invalid_auth_code,
	}

	params := &ClientInitParams{
		Scheme:           target.Scheme,
		Host:             target.Host,
		Port:             target.Port,
		BaseUrl:          target.BaseUrl,
		AuthConfig:       target.AuthConfig,
		ProxyUrl:         target.ProxyUrl,
		VerifySSL:        bool(target.verifySSL),
		VerifySSLUserSet: target.verifySSLUserSet,
		ScrapeTimeout:    time.Duration(target.ScrapeTimeout),
		QueryRetry:       target.QueryRetry,
	}
	cl.symtab["__collector_id"] = target.Name
	cl.Init(params)
	delete(cl.symtab, "__collector_id")

	return cl
}

// ***********************
func (c *Client) Clone(target *TargetConfig) *Client {
	//sync.Mutex{}
	cl := &Client{
		// logContext: []interface{}{},
		logger: c.logger,
		sc:     c.sc,
		// wait_mutex:    sync.Mutex{},
		// content_mutex: sync.Mutex{},
		invalid_auth_code: c.invalid_auth_code,
	}

	var err error
	var tmp any

	tmp = c.symtab
	if tmp, err = copystructure.Copy(c.symtab); err != nil {
		c.logger.Error(
			"can't clone symbols table for new client",
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		return nil
	}
	if val, ok := tmp.(map[string]any); ok {
		cl.symtab = val
	} else {
		cl.symtab = make(map[string]any)
	}

	params := &ClientInitParams{
		Scheme:           target.Scheme,
		Host:             target.Host,
		Port:             target.Port,
		BaseUrl:          target.BaseUrl,
		AuthConfig:       target.AuthConfig,
		ProxyUrl:         target.ProxyUrl,
		VerifySSL:        bool(target.verifySSL),
		VerifySSLUserSet: target.verifySSLUserSet,
		ScrapeTimeout:    time.Duration(target.ScrapeTimeout),
		QueryRetry:       target.QueryRetry,
	}
	cl.Init(params)

	// duplicate headers from source into clone
	for header, values := range c.client.Header {
		cl.client.SetHeader(header, values[0])
	}

	// duplicate cookies from source into clone
	cl.client.SetCookies(c.client.Cookies)

	auth_set, _ := GetMapValueBool(c.symtab, "auth_set")
	if auth_set && c.client.UserInfo != nil {
		cl.client.UserInfo = &resty.User{
			Username: c.client.UserInfo.Username,
			Password: c.client.UserInfo.Password,
		}

		cl.symtab["auth_set"] = true
	}
	return cl
}

// set the url for client
func (c *Client) SetUrl(url string) string {
	if _, ok := c.symtab["APIEndPoint"]; !ok {
		err := fmt.Errorf("http base uri not found")
		c.logger.Error(
			err.Error(),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		return ""
	}
	base := c.symtab["APIEndPoint"].(string)

	uri := fmt.Sprintf("%s/%s", base, strings.TrimPrefix(url, "/"))
	c.symtab["uri"] = uri

	c.logger.Debug(
		"uri set",
		"uri", uri,
		"collid", CollectorId(c.symtab, c.logger),
		"script", ScriptName(c.symtab, c.logger))
	return uri
}

// parse a response to a json map[string]interface{}
func (c *Client) getJSONResponse(resp *resty.Response) any {
	var data any

	body := resp.Body()
	if len(body) > 0 {
		content_type := resp.Header().Get(contentTypeHeader)
		if strings.Contains(content_type, "application/json") {
			// tmp := make([]byte, len(body))
			// copy(tmp, body)
			if err := json.Unmarshal(body, &data); err != nil {
				c.logger.Error(
					fmt.Sprintf("Fail to decode json results %v", err),
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger))
			}
		}
	} else {
		data = make(map[any]any)
	}
	return data
}

// unmarshall a default content in xml... I hope so
func (res *Content) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// var s string
	content := &Content{
		Type: content_node,
	}
	// node := make(map[string]any)
	attrs := make(map[string]any)
	for _, attr := range start.Attr {
		attrs[attr.Name.Local] = attr.Value
	}
	// node[start.Name.Local] = attrs
	content.Name = start.Name.Local
	content.Attrs = attrs
	*res = *content

	for {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch tt := t.(type) {
		case xml.StartElement:
			var (
				sub_obj      *Content
				sub_obj_list []any
			)
			if err := d.DecodeElement(&sub_obj, &tt); err != nil {
				return err
			}
			if sub_obj.Type == content_value {
			}

			if sub_obj.Type == content_value {
				// check if we have already the same attribute set for the parent objet
				// if true: must transform attribute to list of value
				if raw_sl, ok := attrs[sub_obj.Name]; ok {
					if sl, ok := raw_sl.([]any); ok {
						sub_obj_list = sl
					} else {
						sub_obj_list = make([]any, 0)
						sub_obj_list = append(sub_obj_list, raw_sl)
					}
					sub_obj_list = append(sub_obj_list, sub_obj.Attrs[sub_obj.Name])
					attrs[sub_obj.Name] = sub_obj_list
				} else {
					attrs[sub_obj.Name] = sub_obj.Attrs[sub_obj.Name]
				}
			} else {
				if raw_sl, ok := attrs[sub_obj.Name]; !ok {
					sub_obj_list = make([]any, 0)
				} else {
					if sl, ok := raw_sl.([]any); ok {
						sub_obj_list = sl
					}
				}
				sub_obj_list = append(sub_obj_list, sub_obj.Attrs)
				attrs[sub_obj.Name] = sub_obj_list
			}

		case xml.EndElement:
			if tt == start.End() {
				return nil
			}
		case xml.CharData:
			val := tt.Copy()
			val = bytes.Trim(val, "\t\n\r ")
			if len(val) != 0 {
				content.Type = content_value
				content.Attrs[content.Name] = string(val)
				*res = *content
			}
		}
	}
}

// parse a response in XML and return map[string]any
func (c *Client) getXMLResponse(resp *resty.Response) any {
	// var data map[string]interface{}
	// var data_map map[string]interface{}
	var data any

	body := resp.Body()
	if len(body) > 0 {
		content_type := resp.Header().Get(contentTypeHeader)
		if strings.Contains(content_type, "application/json") {
			// tmp := make([]byte, len(body))
			// copy(tmp, body)
			var data_internal *Content
			if err := xml.Unmarshal(body, &data_internal); err != nil {
				c.logger.Error(
					fmt.Sprintf("Fail to decode xml results %v", err),
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger))
			}
			data := make(map[string]any)
			data[data_internal.Name] = data_internal.Attrs
		}
	} else {
		data = make(map[any]any)
	}
	return data
}

// sent HTTP Method to uri with params or body and get the reponse and the json obj
func (c *Client) Execute(
	method, uri string,
	params map[string]string,
	body interface{},
	parser string) (
	// check_invalid_auth bool) (
	*resty.Response,
	any,
	error) {

	var err error
	var data any
	var query_retry int
	var ok bool

	url := c.SetUrl(uri)
	c.logger.Debug(
		"querying httpapi",
		"collid", CollectorId(c.symtab, c.logger),
		"script", ScriptName(c.symtab, c.logger),
		"method", method,
		"url", url,
		"parser", "parser",
	)
	if body != nil {
		c.logger.Debug(
			"querying httpapi",
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger),
			"method", method,
			"url", url,
			"body", fmt.Sprintf("%+v", body))
	}
	if len(params) > 0 {
		c.logger.Debug(
			"querying httpapi",
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger),
			"method", method,
			"url", url,
			"params", params)
	}

	if query_retry, ok = GetMapValueInt(c.symtab, "queryRetry"); !ok {
		query_retry = 1
	}
	var resp *resty.Response

	req := c.client.NewRequest()
	if body != nil {
		req.SetBody(body)
	}
	if len(params) > 0 {
		req.SetQueryParams(params)
	}

	for i := 0; i <= query_retry; i++ {
		resp, err = req.Execute(method, url)
		if err == nil {
			// check if retry and invalid auth to replat Ping() script
			code := resp.StatusCode()
			// if (i+1 < query_retry) && check_invalid_auth && slices.Contains(c.invalid_auth_code, code) {
			if (i+1 < query_retry) && slices.Contains(c.invalid_auth_code, code) {
				c.logger.Debug(
					"received invalid auth. start Ping()/Login()",
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger))

				c.symtab["logged"] = false

				return resp, data, ErrInvalidLogin

			} else {
				if parser == "xml" {
					data = c.getXMLResponse(resp)
				} else if parser == "json" {
					data = c.getJSONResponse(resp)
				}
				i = query_retry + 1
			}
			c.symtab["response_headers"] = resp.Header()
			c.symtab["response_cookies"] = resp.Cookies()
		} else {
			c.logger.Debug(
				fmt.Sprintf("query unsuccessfull: retrying (%d)", i+1),
				"collid", CollectorId(c.symtab, c.logger),
				"script", ScriptName(c.symtab, c.logger),
				"errmsg", err)
			code := resp.StatusCode()
			if code == 599 || strings.Contains(err.Error(), "context deadline exceeded") {
				err = ErrContextDeadLineExceeded
			} else {
				delete(c.symtab, "response_headers")
				delete(c.symtab, "response_cookies")
			}
			break
		}
	}
	// something wrong with retry...
	if resp == nil {
		err = fmt.Errorf("empty response")
	}
	return resp, data, err
}

// add headers to client
func (cl *Client) proceedHeaders() error {

	if r_headers, ok := cl.symtab["headers"]; ok {
		// format: "header" "value"
		var head_name, head_value, action string
		var headers map[any]any
		var err error
		var ok bool
		if headers, ok = r_headers.(map[any]any); ok {
			for r_header, r_value := range headers {
				// ** get header name
				switch header := r_header.(type) {
				case *Field:
					if head_name, err = header.GetValueString(cl.symtab, nil, false); err != nil {
						return err
					}
				case string:
					head_name = header
				}

				switch value := r_value.(type) {
				case *Field:
					if head_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
						return err
					}
				case string:
					head_value = value
				}
				if head_value == "__delete__" || head_value == "__remove__" {
					cl.client.Header.Del(head_name)
				} else {
					cl.client.SetHeader(head_name, head_value)
				}
			}
		} else if s_headers, ok := r_headers.(map[string]any); ok {
			for head_name, r_value := range s_headers {
				// ** get header name
				// nothing: already head_name

				switch value := r_value.(type) {
				case *Field:
					if head_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
						return err
					}
				case string:
					head_value = value
				}
				if head_value == "__delete__" || head_value == "__remove__" {
					cl.client.Header.Del(head_name)
				} else {
					cl.client.SetHeader(head_name, head_value)
				}
			}
		} else if headers_list, ok := r_headers.([]any); ok {
			// format: "- name:header"\n value: "header_value" mode: add|delete
			for _, map_header := range headers_list {
				if headers, ok = map_header.(map[any]any); ok {
					action = "add"
					head_name = ""
					head_value = ""
					for r_key, r_value := range headers {
						var key_name string
						switch key_val := r_key.(type) {
						case *Field:
							if key_name, err = key_val.GetValueString(cl.symtab, nil, false); err != nil {
								return err
							}
						case string:
							key_name = key_val
						}
						if key_name == "name" {
							switch value := r_value.(type) {
							case *Field:
								if head_name, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								head_name = value
							}

						}
						// get value
						if key_name == "value" {
							switch value := r_value.(type) {
							case *Field:
								if head_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								head_value = value
							}

						}

						if key_name == "action" {
							switch value := r_value.(type) {
							case *Field:
								if action, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								action = value
							}
						}
					}
					if head_name != "" && head_value != "" {
						if action == "add" {
							cl.client.SetHeader(head_name, head_value)
						} else if action == "delete" || action == "remove" {
							cl.client.Header.Del(head_name)
						}
					}
				} else if headers, ok := map_header.(map[string]any); ok {
					action = "add"
					head_name = ""
					head_value = ""
					for key_name, r_value := range headers {

						if key_name == "name" {
							switch value := r_value.(type) {
							case *Field:
								if head_name, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								head_name = value
							}

						}
						// get value
						if key_name == "value" {
							switch value := r_value.(type) {
							case *Field:
								if head_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								head_value = value
							}
						}

						if key_name == "action" {
							switch value := r_value.(type) {
							case *Field:
								if action, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								action = value
							}
						}
					}
					if head_name != "" && head_value != "" {
						if action == "add" {
							cl.client.SetHeader(head_name, head_value)
						} else if action == "delete" || action == "remove" {
							cl.client.Header.Del(head_name)
						}
					}
				}
			}
		}
		// reset special var headers from th symbols table
		delete(cl.symtab, "headers")
	}
	return nil
}

func UpdateCookie(cookies []*http.Cookie, cookie *http.Cookie) []*http.Cookie {
	found := false
	for index, http_cookie := range cookies {
		if http_cookie.Name == cookie.Name {
			cookies[index] = cookie
			found = true
			break
		}
	}
	if !found {
		cookies = append(cookies, cookie)
	}
	return cookies
}

func DeleteCookie(cookies []*http.Cookie, cookie_name string) []*http.Cookie {
	for index, http_cookie := range cookies {
		if http_cookie.Name == cookie_name {
			cookies = append(cookies[:index], cookies[index+1:]...)
		}
	}
	return cookies
}

// add cookie to client
func (cl *Client) proceedCookies() error {

	if r_cookies, ok := cl.symtab["cookies"]; ok {
		// format: name: "header" value: "value" path:
		var (
			cookie_name, cookie_value, cookie_path, cookie_domain, action string
			cookie_max_age                                                int
			headers                                                       map[any]any
			err                                                           error
			ok                                                            bool
		)
		if headers, ok = r_cookies.(map[any]any); ok {
			for r_header, r_value := range headers {
				// ** get header name
				switch header := r_header.(type) {
				case *Field:
					if cookie_name, err = header.GetValueString(cl.symtab, nil, false); err != nil {
						return err
					}
				case string:
					cookie_name = header
				}

				switch value := r_value.(type) {
				case *Field:
					if cookie_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
						return err
					}
				case string:
					cookie_value = value
				}
				if cookie_value == "__delete__" || cookie_value == "__remove__" {
					cl.client.Cookies = DeleteCookie(cl.client.Cookies, cookie_name)
				} else {
					cookie := &http.Cookie{
						Name:  cookie_name,
						Value: cookie_value,
					}
					cl.client.Cookies = UpdateCookie(cl.client.Cookies, cookie)
				}
			}
		} else if cookies_list, ok := r_cookies.([]any); ok {
			// format: "- name:header"\n value: "header_value" mode: add|delete
			for _, map_cookie := range cookies_list {
				if headers, ok = map_cookie.(map[any]any); ok {
					action = "add"
					cookie_name = ""
					cookie_value = ""
					cookie_path = ""
					cookie_domain = ""
					cookie_max_age = -1
					for r_key, r_value := range headers {
						var key_name string
						switch key_val := r_key.(type) {
						case *Field:
							if key_name, err = key_val.GetValueString(cl.symtab, nil, false); err != nil {
								return err
							}
						case string:
							key_name = key_val
						}
						if key_name == "name" {
							switch value := r_value.(type) {
							case *Field:
								if cookie_name, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_name = value
							}
						} else if key_name == "value" {
							// get value
							switch value := r_value.(type) {
							case *Field:
								if cookie_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_value = value
							}
						} else if key_name == "domain" {
							// get domain
							switch value := r_value.(type) {
							case *Field:
								if cookie_domain, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_domain = value
							}
						} else if key_name == "path" {
							// get path
							switch value := r_value.(type) {
							case *Field:
								if cookie_path, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_path = value
							}
						} else if key_name == "action" {
							switch value := r_value.(type) {
							case *Field:
								if action, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								action = value
							}
						}
					}
					if cookie_name != "" {
						if action == "add" && cookie_value != "" {
							cookie := &http.Cookie{
								Name:  cookie_name,
								Value: cookie_value,
							}
							if cookie_path != "" {
								cookie.Path = cookie_path
							}
							if cookie_domain != "" {
								cookie.Domain = cookie_domain
							}
							if cookie_max_age != -1 {
								cookie.MaxAge = cookie_max_age
							}

							cl.client.Cookies = UpdateCookie(cl.client.Cookies, cookie)
						} else if action == "delete" || action == "remove" {
							cl.client.Cookies = DeleteCookie(cl.client.Cookies, cookie_name)
						}
					}
				} else if headers, ok := map_cookie.(map[string]any); ok {
					action = "add"
					cookie_name = ""
					cookie_value = ""
					cookie_path = ""
					cookie_domain = ""
					cookie_max_age = -1
					for key_name, r_value := range headers {
						if key_name == "name" {
							switch value := r_value.(type) {
							case *Field:
								if cookie_name, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_name = value
							}
						} else if key_name == "value" {
							// get value
							switch value := r_value.(type) {
							case *Field:
								if cookie_value, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_value = value
							}
						} else if key_name == "domain" {
							// get domain
							switch value := r_value.(type) {
							case *Field:
								if cookie_domain, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_domain = value
							}
						} else if key_name == "path" {
							// get path
							switch value := r_value.(type) {
							case *Field:
								if cookie_path, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								cookie_path = value
							}
						} else if key_name == "action" {
							switch value := r_value.(type) {
							case *Field:
								if action, err = value.GetValueString(cl.symtab, nil, false); err != nil {
									return err
								}
							case string:
								action = value
							}
						}
					}
					if cookie_name != "" {
						if action == "add" && cookie_value != "" {
							cookie := &http.Cookie{
								Name:  cookie_name,
								Value: cookie_value,
							}
							if cookie_path != "" {
								cookie.Path = cookie_path
							}
							if cookie_domain != "" {
								cookie.Domain = cookie_domain
							}
							if cookie_max_age != -1 {
								cookie.MaxAge = cookie_max_age
							}

							cl.client.Cookies = UpdateCookie(cl.client.Cookies, cookie)
						} else if action == "delete" || action == "remove" {
							cl.client.Cookies = DeleteCookie(cl.client.Cookies, cookie_name)
						}
					}
				}
			}
		}

		// reset cookies var in symbols table
		delete(cl.symtab, "cookies")
	}
	return nil
}

type CallClientExecuteParams struct {
	Payload  string
	Method   string
	Url      string
	Debug    bool
	VarName  string
	OkStatus []int
	AuthMode string
	Username string
	Password string
	Token    string
	Timeout  time.Duration
	Parser   string
	// Check_invalid_Auth bool
}

func (c *Client) callClientExecute(params *CallClientExecuteParams, symtab map[string]any) error {
	var (
		payload any
	)
	if params.Payload == "" {
		payload = nil
	} else {
		payload = params.Payload
	}

	if params.Method == "" {
		err := fmt.Errorf("http method not found")
		c.logger.Error(
			err.Error(),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		return err
	}
	method := strings.ToUpper(params.Method)

	if params.Url == "" {
		err := fmt.Errorf("http url not found")
		c.logger.Error(
			err.Error(),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		return err
	}
	url := params.Url
	old_values := make(map[string]string, 4)

	auth_mode := GetMapValueString(symtab, "auth_mode")
	if params.AuthMode != "" {
		old_values["auth_mode"] = auth_mode
		auth_mode = params.AuthMode
		symtab["auth_mode"] = auth_mode
	}

	if params.Timeout != 0 {
		old_values["timeout"] = fmt.Sprintf("%d", c.client.GetClient().Timeout)
		c.client.SetTimeout(params.Timeout)
	}

	auth_set, _ := GetMapValueBool(symtab, "auth_set")
	if !auth_set {
		if auth_mode == "basic" {
			user := GetMapValueString(symtab, "user")
			if params.Username != "" {
				old_values["user"] = user
				user = params.Username
				symtab["user"] = user
			}
			passwd := GetMapValueString(symtab, "password")
			if params.Password != "" {
				old_values["password"] = passwd
				passwd = params.Password
				symtab["password"] = passwd
			}
			if strings.HasPrefix(passwd, "/encrypted/") {
				ciphertext := passwd[len("/encrypted/"):]
				c.logger.Debug(
					"encrypted password detected",
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger),
					"ciphertext", ciphertext)
				auth_key := GetMapValueString(symtab, "auth_key")
				c.logger.Debug(
					"auth_key detected",
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger),
					"auth_key", auth_key)
				cipher, err := encrypt.NewAESCipher(auth_key)
				if err != nil {
					err := ErrInvalidLoginNoCipher
					// level.Error(c.logger).Log("errmsg", err)
					return err
				}
				passwd, err = cipher.Decrypt(ciphertext, true)
				if err != nil {
					err := ErrInvalidLoginInvalidCipher
					// level.Error(c.logger).Log("errmsg", err)
					return err
				}
			}
			c.client.SetBasicAuth(user, passwd)
			passwd = ""
			symtab["auth_set"] = true
			c.logger.Debug(
				"basicauth Header set for request",
				"collid", CollectorId(c.symtab, c.logger),
				"script", ScriptName(c.symtab, c.logger))
			delete(symtab, "auth_key")
		} else if auth_mode == "token" {
			auth_token := GetMapValueString(symtab, "auth_token")
			if params.Token != "" {
				old_values["auth_token"] = auth_token
				auth_token = params.Token
				symtab["auth_token"] = auth_token
			}
			if auth_token != "" {
				c.client.SetAuthToken(auth_token)
				c.logger.Debug(
					"token Hearder set for request",
					"collid", CollectorId(c.symtab, c.logger),
					"script", ScriptName(c.symtab, c.logger))
			}
		}
	}

	if len(params.OkStatus) <= 0 {
		err := fmt.Errorf("ok_status not found")
		c.logger.Error(
			err.Error(),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		return err
	}
	valid_status := params.OkStatus

	var_name := params.VarName

	// set local headers and cookies map
	if err := c.proceedHeaders(); err != nil {
		return err
	}
	if err := c.proceedCookies(); err != nil {
		return err
	}

	//******************
	//* play the request
	// resp, data, err := c.Execute(method, url, nil, payload, params.Check_invalid_Auth)
	resp, data, err := c.Execute(method, url, nil, payload, params.Parser)
	if err != nil {
		return err
	}
	if params.Debug {
		c.logger.Debug(
			"launch query debug",
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger),
			"url", symtab["uri"].(string),
			"results", string(resp.Body()))
	}
	// * get returned status
	code := resp.StatusCode()
	// * set it to symbols table so user can access it
	symtab["results_status"] = code

	if !slices.Contains(valid_status, code) {
		symtab["query_status"] = false
		c.logger.Info(
			fmt.Sprintf("invalid response status: (%d not in %v)",
				code, valid_status),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger))
		c.logger.Debug(
			fmt.Sprintf("invalid req headers: (%v) req cookies %v- response headers: (%v)",
				c.client.Header, c.client.Cookies, resp.Header()),
			"collid", CollectorId(c.symtab, c.logger),
			"script", ScriptName(c.symtab, c.logger),
		)

		err = ErrInvalidQueryResult
	} else {
		if data == nil {
			err = fmt.Errorf("fail to decode json results: %v", err)
			return err
		} else {
			if var_name != "" && var_name != "_" {
				symtab[var_name] = data
			} else if var_name == "_root" {
				opts := mergo.WithOverride
				if err := mergo.Merge(&symtab, data, opts); err != nil {
					c.logger.Error(
						"merging results into symbols table",
						"collid", CollectorId(c.symtab, c.logger),
						"script", ScriptName(c.symtab, c.logger),
						"errmsg", err)
					return err
				}
			}
			symtab["query_status"] = true

			err = nil
		}
	}
	// reset local auth param from client
	if auth_mode, ok := old_values["auth_mode"]; ok {
		symtab["auth_mode"] = auth_mode
		if params.AuthMode == "basic" && params.AuthMode != auth_mode {
			c.client.UserInfo = nil
		}
	}

	if user, ok := old_values["user"]; ok {
		symtab["user"] = user
	}

	if passwd, ok := old_values["passwd"]; ok {
		symtab["passwd"] = passwd
	}

	if auth_token, ok := old_values["auth_token"]; ok {
		symtab["auth_token"] = auth_token
		if params.Token != auth_token {
			c.client.Token = ""
		}
	}

	if timeout_str, ok := old_values["timeout"]; ok {
		var i_value int64
		if i_value, err = strconv.ParseInt(timeout_str, 10, 0); err != nil {
			i_value = 0
		}
		c.client.SetTimeout(time.Duration(i_value))
	}

	return err
}

func GetMapValueString(symtab map[string]any, key string) string {
	var value string
	if value_raw, ok := symtab[key]; ok {
		switch value_val := value_raw.(type) {
		case string:
			value = value_val
		case int:
			value = fmt.Sprintf("%d", value_val)
		default:
			value = ""
		}
	}
	return value
}

func GetMapValueInt(symtab map[string]any, key string) (int, bool) {
	var value int
	found := false
	if value_raw, ok := symtab[key]; ok {
		found = true
		switch value_val := value_raw.(type) {
		case string:
			var i_value int64
			var err error
			if i_value, err = strconv.ParseInt(value_val, 10, 0); err != nil {
				i_value = 0
			}
			value = int(i_value)
		case int:
			value = value_val
		default:
			value = 0
			found = false
		}
	}
	return value, found
}

func GetMapValueBool(symtab map[string]any, key string) (bool, bool) {
	var value bool

	found := false
	if value_raw, ok := symtab[key]; ok {
		found = true
		switch value_val := value_raw.(type) {
		case bool:
			value = value_val
		case string:
			asString := strings.ToLower(value_val)
			if asString == "1" || asString == "true" || asString == "yes" || asString == "on" {
				value = true
			} else if asString == "0" || asString == "false" || asString == "no" || asString == "off" {
				value = false
			}
		default:
			value = false
			found = false
		}
	}
	return value, found
}

// ****************************************************************
// user HTTP connections script steps
// init(): to initialize http request
// login(): to login to the http API and proceed result (token bearer)
// logout(): to logout and reset parameters
// ping(): to check the auth/cnx is still active
type ClientInitParams struct {
	Scheme     string
	Host       string
	Port       string
	BaseUrl    string
	AuthConfig AuthConfig
	// BasicAuth         bool
	// Username          string
	// Password          Secret
	ProxyUrl         string
	VerifySSL        bool
	VerifySSLUserSet bool
	ScrapeTimeout    time.Duration
	QueryRetry       int
}

func (cl *Client) Init(params *ClientInitParams) error {
	var reset_coll_id bool = false

	defer func() {
		if reset_coll_id {
			delete(cl.symtab, "__collector_id")
		}
	}()

	// ** get the init script definition from config if one is defined
	// ** set default config for all targets
	if script, ok := cl.sc["init"]; ok && script != nil {
		// cl.symtab["__client"] = cl.client
		cl.symtab["__method"] = cl.callClientExecute
		cid := GetMapValueString(cl.symtab, "__collector_id")
		if cid == "" {
			cl.symtab["__collector_id"] = "--"
			reset_coll_id = true
		}
		err := script.Play(cl.symtab, false, cl.logger)
		delete(cl.symtab, "__method")

		if err != nil {
			return err
		}
	}
	var base_url, scheme, port string
	var verifySSL bool
	var query_retry int

	base_url = GetMapValueString(cl.symtab, "base_url")
	scheme = GetMapValueString(cl.symtab, "scheme")
	port = GetMapValueString(cl.symtab, "port")
	verifySSL, _ = GetMapValueBool(cl.symtab, "verifySSL")
	query_retry, _ = GetMapValueInt(cl.symtab, "queryRetry")

	// ** update default parameters with target parameters
	if params.BaseUrl != "" {
		base_url = params.BaseUrl
	}
	if params.Scheme != "" {
		scheme = params.Scheme
	}
	if params.Port != "" {
		port = params.Port
	}
	// WARNING !!! params has priority over init.
	if verifySSL != params.VerifySSL && params.VerifySSLUserSet {
		verifySSL = params.VerifySSL
	}
	if query_retry != params.QueryRetry {
		query_retry = params.QueryRetry
	}
	apiendpoint := fmt.Sprintf("%s://%s:%s", scheme, params.Host, port)
	baseurl := strings.TrimPrefix(base_url, "/")
	if baseurl != "" {
		apiendpoint += "/" + baseurl
	}

	cl.symtab["APIEndPoint"] = apiendpoint
	cl.symtab["scheme"] = scheme
	cl.symtab["host"] = params.Host
	cl.symtab["port"] = port
	cl.symtab["base_url"] = base_url
	cl.symtab["auth_mode"] = params.AuthConfig.Mode
	cl.symtab["user"] = params.AuthConfig.Username
	cl.symtab["password"] = string(params.AuthConfig.Password)
	cl.symtab["auth_token"] = string(params.AuthConfig.Token)
	cl.symtab["auth_key"] = string(params.AuthConfig.authKey)
	cl.symtab["auth_set"] = false
	cl.symtab["verifySSL"] = verifySSL
	cl.symtab["proxyUrl"] = params.ProxyUrl
	cl.symtab["timeout"] = params.ScrapeTimeout
	cl.symtab["queryRetry"] = query_retry

	if scheme == "https" {
		cl.logger.Debug(
			fmt.Sprintf("verify certificate set to %v", verifySSL),
			"collid", CollectorId(cl.symtab, cl.logger),
			"script", ScriptName(cl.symtab, cl.logger))
		cl.client = resty.New().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: !verifySSL})
	} else if scheme == "http" {
		cl.client = resty.New()
	} else {
		cl.logger.Error(
			fmt.Sprintf("invalid scheme for url '%s'", scheme),
			"collid", CollectorId(cl.symtab, cl.logger),
			"script", ScriptName(cl.symtab, cl.logger))
		return nil
	}
	timeout := time.Duration(cl.symtab["timeout"].(time.Duration))
	cl.client.SetTimeout(timeout)

	if err := cl.proceedHeaders(); err != nil {
		return err
	}
	if err := cl.proceedCookies(); err != nil {
		return err
	}

	if params.AuthConfig.Mode == "basic" {
		passwd := string(params.AuthConfig.Password)
		if params.AuthConfig.Username != "" && passwd != "" &&
			!strings.Contains(passwd, "/encrypted/") {
			cl.client.SetBasicAuth(params.AuthConfig.Username, passwd)
		}
	} else if params.AuthConfig.Mode == "token" && params.AuthConfig.Token != "" {
		token := GetMapValueString(cl.symtab, "auth_token")
		cl.client.SetAuthToken(token)
	}
	if params.ProxyUrl != "" {
		cl.client.SetProxy(params.ProxyUrl)
	}
	// remove http.CookieJar
	cl.client.SetCookieJar(nil)

	return nil
}

// login to target
func (cl *Client) Login() (bool, error) {

	// ** init the connection status func and symbol table
	status := false
	cl.symtab["logged"] = false

	// ** get the login script definition from config if one is defined
	if script, ok := cl.sc["login"]; ok && script != nil {
		// cl.symtab["__client"] = cl.client
		cl.symtab["__method"] = cl.callClientExecute

		err := script.Play(cl.symtab, false, cl.logger)
		delete(cl.symtab, "__method")

		if err != nil {
			return false, err
		}
		if err := cl.proceedHeaders(); err != nil {
			return false, err
		}
		if err := cl.proceedCookies(); err != nil {
			return false, err
		}
		// check is user has set a token
		token := GetMapValueString(cl.symtab, "auth_token")
		if cl.client.Token != token {
			cl.client.SetAuthToken(token)
		}
	} else {
		// * no user script has been defined: logged is equivalent to "ping()" query_status
		cl.symtab["logged"] = cl.symtab["query_status"]
	}

	if logged, ok := GetMapValueBool(cl.symtab, "logged"); ok {
		status = logged
	}

	return status, nil
}

// logout from target
func (cl *Client) Logout() error {
	// ** get the login script definition from config if one is defined
	if script, ok := cl.sc["logout"]; ok && script != nil {
		// cl.symtab["__client"] = cl.client
		cl.symtab["__method"] = cl.callClientExecute
		cl.symtab["__collector_id"] = "tg"

		err := script.Play(cl.symtab, false, cl.logger)
		delete(cl.symtab, "__collector_id")
		delete(cl.symtab, "__method")

		if err != nil {
			return err
		}

		if err := cl.proceedHeaders(); err != nil {
			return err
		}
		if err := cl.proceedCookies(); err != nil {
			return err
		}
	} else {
		// ** no user script found: equivalent to Clear()
		return cl.Clear()
	}
	return nil
}

// clear auth info for target
func (cl *Client) Clear() error {
	// ** get the clear script definition from config if one is defined
	if script, ok := cl.sc["clear"]; ok && script != nil {
		// cl.symtab["__client"] = cl.client
		cl.symtab["__method"] = cl.callClientExecute
		// cl.symtab["__collector_id"] = "tg"

		err := script.Play(cl.symtab, false, cl.logger)
		// delete(cl.symtab, "__collector_id")
		delete(cl.symtab, "__method")

		if err != nil {
			return err
		}
	} else {
		cl.symtab["logged"] = false
		cl.symtab["auth_set"] = false
		delete(cl.symtab, "auth_token")
		cl.client.SetAuthToken("")
	}

	if err := cl.proceedHeaders(); err != nil {
		return err
	}
	if err := cl.proceedCookies(); err != nil {
		return err
	}
	return nil
}

// ping the target
func (cl *Client) Ping() (bool, error) {

	// ** init the connection status func and symbol table
	status := false
	cl.symtab["query_status"] = false

	cl.content_mutex.Lock()
	logger := cl.logger
	cl.content_mutex.Unlock()

	// ** get the ping script definition from config if one is defined
	if script, ok := cl.sc["ping"]; ok && script != nil {
		logger.Debug(
			fmt.Sprintf("starting script '%s'", script.name),
			"collid", CollectorId(cl.symtab, logger),
			"script", ScriptName(cl.symtab, logger))
		// cl.symtab["__client"] = cl.client

		cl.symtab["__method"] = cl.callClientExecute
		// cl.symtab["check_invalid_auth"] = false
		err := script.Play(cl.symtab, false, logger)
		delete(cl.symtab, "__method")
		// delete(cl.symtab, "check_invalid_auth")

		if err != nil {
			return false, err
		}
	} else {
		err := fmt.Errorf("no ping script found... can't connect")
		logger.Error(
			err.Error(),
			"collid", CollectorId(cl.symtab, logger),
			"script", ScriptName(cl.symtab, logger))
		return false, err
	}

	if query_status, ok := GetMapValueBool(cl.symtab, "query_status"); ok {
		status = query_status
	}

	if err := cl.proceedHeaders(); err != nil {
		return status, err
	}
	if err := cl.proceedCookies(); err != nil {
		return status, err
	}

	// check is user has set a token
	token := GetMapValueString(cl.symtab, "auth_token")
	if cl.client.Token != token {
		cl.client.SetAuthToken(token)
	}

	return status, nil

}
