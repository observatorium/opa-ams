package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type roundTripperInstrumenter struct {
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func newRoundTripperInstrumenter(r prometheus.Registerer) *roundTripperInstrumenter {
	ins := &roundTripperInstrumenter{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "client_api_requests_total",
				Help: "A counter for requests from the wrapped client.",
			},
			[]string{"code", "method", "client"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_duration_seconds",
				Help:    "A histogram of request latencies.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "client"},
		),
	}

	if r != nil {
		r.MustRegister(
			ins.requestCounter,
			ins.requestDuration,
		)
	}

	return ins
}

// NewRoundTripper wraps a HTTP RoundTripper with some metrics.
func (i *roundTripperInstrumenter) NewRoundTripper(name string, rt http.RoundTripper) http.RoundTripper {
	counter := i.requestCounter.MustCurryWith(prometheus.Labels{"client": name})
	duration := i.requestDuration.MustCurryWith(prometheus.Labels{"client": name})

	return promhttp.InstrumentRoundTripperCounter(counter,
		promhttp.InstrumentRoundTripperDuration(duration, rt),
	)
}

type handlerInstrumenter struct {
	requestCounter  *prometheus.CounterVec
	requestSize     *prometheus.SummaryVec
	requestDuration *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
}

func newHandlerInstrumenter(r prometheus.Registerer) *handlerInstrumenter {
	labels := []string{"code", "method", "handler"}
	ins := &handlerInstrumenter{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Counter of HTTP requests.",
			},
			labels,
		),
		requestSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_request_size_bytes",
				Help: "Size of HTTP requests.",
			},
			labels,
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Histogram of latencies for HTTP requests.",
				Buckets: []float64{.1, .2, .4, 1, 2.5, 5, 8, 20, 60, 120},
			},
			labels,
		),
		responseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "Histogram of response size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8), //nolint:gomnd
			},
			labels,
		),
	}

	if r != nil {
		r.MustRegister(
			ins.requestCounter,
			ins.requestSize,
			ins.requestDuration,
			ins.responseSize,
		)
	}

	return ins
}

// NewHandler wraps a HTTP handler with some metrics for HTTP handlers.
func (i *handlerInstrumenter) NewHandler(labels prometheus.Labels, handler http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(i.requestCounter.MustCurryWith(labels),
		promhttp.InstrumentHandlerRequestSize(i.requestSize.MustCurryWith(labels),
			promhttp.InstrumentHandlerDuration(i.requestDuration.MustCurryWith(labels),
				promhttp.InstrumentHandlerResponseSize(i.responseSize.MustCurryWith(labels),
					handler,
				),
			),
		),
	)
}
