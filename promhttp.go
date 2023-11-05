package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

const (
	contentTypeHeader     = "Content-Type"
	contentLengthHeader   = "Content-Length"
	contentEncodingHeader = "Content-Encoding"
	acceptEncodingHeader  = "Accept-Encoding"
)

// ExporterHandlerFor returns an http.Handler for the provided Exporter.
func ExporterHandlerFor(exporter Exporter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var (
			err    error
			target Target
			tmp_t  *TargetConfig
		)
		params := req.URL.Query()

		tname := params.Get("target")
		if tname == "" {
			err := fmt.Errorf("Target parameter is missing")
			HandleError(http.StatusBadRequest, err, *metricsPath, exporter, w, req)
			return
		}
		target, err = exporter.FindTarget(tname)
		if err == ErrTargetNotFound {
			model := params.Get("model")
			if model == "" {
				model = "default"
			}
			t_def, err := exporter.FindTarget(model)
			if err != nil {
				err := fmt.Errorf("Target model '%s' not found: %s", model, err)
				HandleError(http.StatusNotFound, err, *metricsPath, exporter, w, req)
				return
			}
			if tmp_t, err = t_def.Config().Clone(tname, ""); err != nil {
				err := fmt.Errorf("invalid url set for remote_target '%s' %s", tname, err)
				HandleError(http.StatusInternalServerError, err, *metricsPath, exporter, w, req)
				return
			}
			target, err = exporter.AddTarget(tmp_t)
			if err != nil {
				err := fmt.Errorf("unable to create temporary target %s", err)
				HandleError(http.StatusInternalServerError, err, *metricsPath, exporter, w, req)
				return
			}
			exporter.Config().Targets = append(exporter.Config().Targets, tmp_t)
		} else if err != nil {
			HandleError(http.StatusNotFound, err, *metricsPath, exporter, w, req)
			return
		}

		// set authentication for target if one is specified and it differs from target internal
		auth_name := params.Get("auth_name")
		if auth_name != "" && target.Config().AuthName != auth_name {
			auth := exporter.Config().FindAuthConfig(auth_name)
			if auth != nil {
				target.Config().AuthConfig = *auth
				target.SetSymbol("auth_mode", auth.Mode)
				target.SetSymbol("user", auth.Username)
				target.SetSymbol("password", string(auth.Password))
				target.SetSymbol("auth_token", string(auth.Token))
				target.Config().AuthName = auth_name
			}
		}

		auth_key := params.Get("auth_key")
		if auth_key != "" {
			target.SetSymbol("auth_key", auth_key)
		}
		ctx, cancel := contextFor(req, exporter, target)
		defer func() {
			cancel()
		}()

		// Go through prometheus.Gatherers to sanitize and sort metrics.
		gatherer := prometheus.Gatherers{exporter.WithContext(ctx, target)}
		mfs, err := gatherer.Gather()
		if err != nil {
			level.Error(exporter.Logger()).Log("msg", fmt.Sprintf("Error gathering metrics for '%s': %s", tname, err))
			if len(mfs) == 0 {
				http.Error(w, "No metrics gathered, "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		contentType := expfmt.Negotiate(req.Header)
		buf := getBuf()
		defer giveBuf(buf)
		writer, encoding := decorateWriter(req, buf)
		enc := expfmt.NewEncoder(writer, contentType)
		var errs prometheus.MultiError
		for _, mf := range mfs {
			if err := enc.Encode(mf); err != nil {
				errs = append(errs, err)
				level.Info(exporter.Logger()).Log("msg", fmt.Sprintf("Error encoding metric family %q: %s", mf.GetName(), err))
			}
		}
		if closer, ok := writer.(io.Closer); ok {
			closer.Close()
		}
		if errs.MaybeUnwrap() != nil && buf.Len() == 0 {
			err = fmt.Errorf("no metrics encoded: %s, ", errs.Error())
			HandleError(http.StatusInternalServerError, err, *metricsPath, exporter, w, req)
			return
		}
		header := w.Header()
		header.Set(contentTypeHeader, string(contentType))
		header.Set(contentLengthHeader, fmt.Sprint(buf.Len()))
		if encoding != "" {
			header.Set(contentEncodingHeader, encoding)
		}
		w.Write(buf.Bytes())
	})
}

func contextFor(req *http.Request, exporter Exporter, target Target) (context.Context, context.CancelFunc) {
	timeout := time.Duration(0)
	configTimeout := time.Duration(target.Config().ScrapeTimeout)
	// If a timeout is provided in the Prometheus header, use it.
	if v := req.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
		timeoutSeconds, err := strconv.ParseFloat(v, 64)
		if err != nil {
			level.Error(exporter.Logger()).Log("msg", fmt.Sprintf("Failed to parse timeout (`%s`) from Prometheus header: %s", v, err))
		} else {
			timeout = time.Duration(timeoutSeconds * float64(time.Second))

			// Subtract the timeout offset, unless the result would be negative or zero.
			timeoutOffset := time.Duration(exporter.Config().Globals.TimeoutOffset)
			if timeoutOffset > timeout {
				level.Error(exporter.Logger()).Log("msg", fmt.Sprintf("global.scrape_timeout_offset (`%s`) is greater than Prometheus' scraping timeout (`%s`), ignoring",
					timeoutOffset, timeout))
			} else {
				timeout -= timeoutOffset
			}
		}
	}

	// If the configured scrape timeout is more restrictive, use that instead.
	if configTimeout > 0 && (timeout <= 0 || configTimeout < timeout) {
		timeout = configTimeout
	}

	if timeout <= 0 {
		target.SetDeadline(time.Time{})
		return context.Background(), func() {}
	}
	level.Debug(exporter.Logger()).Log("msg", fmt.Sprintf("launching exporter.Gather() with timeout `%s`", timeout))
	target.SetDeadline(time.Now().Add(timeout))

	return context.WithDeadline(context.Background(), target.GetDeadline())
	// return context.WithTimeout(context.Background(), timeout)
}

var bufPool sync.Pool

func getBuf() *bytes.Buffer {
	buf := bufPool.Get()
	if buf == nil {
		return &bytes.Buffer{}
	}
	return buf.(*bytes.Buffer)
}

func giveBuf(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

// decorateWriter wraps a writer to handle gzip compression if requested.  It
// returns the decorated writer and the appropriate "Content-Encoding" header
// (which is empty if no compression is enabled).
func decorateWriter(request *http.Request, writer io.Writer) (w io.Writer, encoding string) {
	header := request.Header.Get(acceptEncodingHeader)
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part := strings.TrimSpace(part)
		if part == "gzip" || strings.HasPrefix(part, "gzip;") {
			return gzip.NewWriter(writer), "gzip"
		}
	}
	return writer, ""
}
