package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/efficientgo/core/merrors"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/metalmatze/signal/healthcheck"
	"github.com/metalmatze/signal/internalserver"
	"github.com/metalmatze/signal/server/signalhttp"
	"github.com/observatorium/api/rbac"
	"github.com/observatorium/api/tracing"
	"github.com/oklog/run"
	"github.com/openshift/telemeter/pkg/authorize/tollbooth"
	"github.com/openshift/telemeter/pkg/cache"
	"github.com/openshift/telemeter/pkg/cache/memcached"
	"github.com/prometheus/client_golang/prometheus"
	flag "github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/observatorium/opa-ams/ams"
)

const (
	dataEndpoint = "/v1/data"
)

var (
	validRule    = regexp.MustCompile("^[_A-Za-z][\\w]*$")
	validPackage = regexp.MustCompile("^[_A-Za-z][\\w]*(\\.[_A-Za-z][\\w]*)*$")
)

type config struct {
	amsURL             string
	logLevel           level.Option
	logFormat          string
	mappings           map[string][]string
	name               string
	resourceTypePrefix string

	oidc      oidcConfig
	opa       opaConfig
	memcached memcachedConfig
	server    serverConfig
	tracing   tracingConfig
}

type opaConfig struct {
	pkg  string
	rule string
}

type serverConfig struct {
	listen         string
	listenInternal string
	healthcheckURL string
}

type memcachedConfig struct {
	expire   int32
	interval int32
	servers  []string
}

type oidcConfig struct {
	audience     string
	clientID     string
	clientSecret string
	issuerURL    string
}

type tracingConfig struct {
	serviceName      string
	endpoint         string
	endpointType     tracing.EndpointType
	samplingFraction float64
}

func parseFlags() (*config, error) {
	var rawTracingEndpointType string

	cfg := &config{}
	flag.StringVar(&cfg.name, "debug.name", "opa-ams", "A name to add as a prefix to log lines.")
	flag.StringVar(&cfg.resourceTypePrefix, "resource-type-prefix", "", "A prefix to add to the resource name in AMS access review requests.")
	logLevelRaw := flag.String("log.level", "info", "The log filtering level. Options: 'error', 'warn', 'info', 'debug'.")
	flag.StringVar(&cfg.logFormat, "log.format", "logfmt", "The log format to use. Options: 'logfmt', 'json'.")
	flag.StringVar(&cfg.server.listen, "web.listen", ":8080", "The address on which the public server listens.")
	flag.StringVar(&cfg.server.listenInternal, "web.internal.listen", ":8081", "The address on which the internal server listens.")
	flag.StringVar(&cfg.server.healthcheckURL, "web.healthchecks.url", "http://localhost:8080", "The URL against which to run healthchecks.")
	flag.StringVar(&cfg.amsURL, "ams.url", "", "An AMS URL against which to authorize client requests.")
	mappingsRaw := flag.StringSlice("ams.mappings", nil, "A list of comma-separated mappings from Observatorium tenants to AMS organization IDs, e.g. foo=bar,x=y")
	mappingsPath := flag.String("ams.mappings-path", "", "A path to a JSON file containing a map from Observatorium tenants to AMS organization IDs.")
	flag.StringVar(&cfg.tracing.serviceName, "internal.tracing.service-name", "opa-ams",
		"The service name to report to the tracing backend.")
	flag.StringVar(&cfg.tracing.endpoint, "internal.tracing.endpoint", "",
		"The full URL of the trace agent or collector. If it's not set, tracing will be disabled.")
	flag.StringVar(&rawTracingEndpointType, "internal.tracing.endpoint-type", string(tracing.EndpointTypeAgent),
		fmt.Sprintf("The tracing endpoint type. Options: '%s', '%s'.", tracing.EndpointTypeAgent, tracing.EndpointTypeCollector))
	flag.Float64Var(&cfg.tracing.samplingFraction, "internal.tracing.sampling-fraction", 0.1,
		"The fraction of traces to sample. Thus, if you set this to .5, half of traces will be sampled.")
	flag.StringVar(&cfg.oidc.issuerURL, "oidc.issuer-url", "", "The OIDC issuer URL, see https://openid.net/specs/openid-connect-discovery-1_0.html#IssuerDiscovery.")
	flag.StringVar(&cfg.oidc.clientSecret, "oidc.client-secret", "", "The OIDC client secret, see https://tools.ietf.org/html/rfc6749#section-2.3.")
	flag.StringVar(&cfg.oidc.clientID, "oidc.client-id", "", "The OIDC client ID, see https://tools.ietf.org/html/rfc6749#section-2.3.")
	flag.StringVar(&cfg.oidc.audience, "oidc.audience", "", "The audience for whom the access token is intended, see https://openid.net/specs/openid-connect-core-1_0.html#IDToken.")
	flag.StringSliceVar(&cfg.memcached.servers, "memcached", nil, "One or more Memcached server addresses.")
	flag.Int32Var(&cfg.memcached.expire, "memcached.expire", 60*60, "Time after which keys stored in Memcached should expire, given in seconds.")
	flag.Int32Var(&cfg.memcached.interval, "memcached.interval", 10, "The interval at which to update the Memcached DNS, given in seconds; use 0 to disable.")
	flag.StringVar(&cfg.opa.pkg, "opa.package", "", "The name of the OPA package that opa-ams should implement, see https://www.openpolicyagent.org/docs/latest/policy-language/#packages.")
	flag.StringVar(&cfg.opa.rule, "opa.rule", "allow", "The name of the OPA rule for which opa-ams should provide a result, see https://www.openpolicyagent.org/docs/latest/policy-language/#rules.")

	flag.Parse()

	switch *logLevelRaw {
	case "error":
		cfg.logLevel = level.AllowError()
	case "warn":
		cfg.logLevel = level.AllowWarn()
	case "info":
		cfg.logLevel = level.AllowInfo()
	case "debug":
		cfg.logLevel = level.AllowDebug()
	default:
		return nil, fmt.Errorf("unexpected log level: %s", *logLevelRaw)
	}

	if len(cfg.opa.pkg) > 0 && !validPackage.Match([]byte(cfg.opa.pkg)) {
		return nil, fmt.Errorf("invalid OPA package name: %s", cfg.opa.pkg)
	}

	if len(cfg.opa.rule) > 0 && !validRule.Match([]byte(cfg.opa.rule)) {
		return nil, fmt.Errorf("invalid OPA rule name: %s", cfg.opa.rule)
	}

	cfg.mappings = make(map[string][]string)
	for _, m := range *mappingsRaw {
		parts := strings.Split(m, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping: %q", m)
		}
		cfg.mappings[parts[0]] = append(cfg.mappings[parts[0]], parts[1])
	}

	if len(*mappingsPath) > 0 {
		buf, err := os.ReadFile(*mappingsPath)
		if err != nil {
			stdlog.Fatalf("unable to read JSON file: %v", err)
		}

		if err := json.Unmarshal(buf, &cfg.mappings); err != nil {
			stdlog.Fatalf("unable to parse contents of %s: %v", *mappingsPath, err)
		}
	}

	cfg.tracing.endpointType = tracing.EndpointType(rawTracingEndpointType)

	return cfg, nil
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		stdlog.Fatal(err)
	}

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))

	if cfg.logFormat == "json" {
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	}

	logger = level.NewFilter(logger, cfg.logLevel)

	if cfg.name != "" {
		logger = log.With(logger, "name", cfg.name)
	}

	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	defer level.Info(logger).Log("msg", "exiting")

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)

	tp, closer, err := tracing.InitTracer(
		cfg.tracing.serviceName,
		cfg.tracing.endpoint,
		cfg.tracing.endpointType,
		cfg.tracing.samplingFraction,
	)
	if err != nil {
		stdlog.Fatalf("initialize tracer: %v", err)
	}

	defer closer()

	otel.SetErrorHandler(otelErrorHandler{logger: logger})

	hi := signalhttp.NewHandlerInstrumenter(reg, []string{"handler"})
	rti := newRoundTripperInstrumenter(reg)

	healthchecks := healthcheck.NewMetricsHandler(healthcheck.NewHandler(), reg)

	amsURL, err := url.Parse(cfg.amsURL)
	if err != nil {
		stdlog.Fatalf("invalid AMS URL: %v", err)
	}
	amsURL.Path = path.Join(amsURL.Path, ams.AccessReviewEndpoint)

	provider, err := oidc.NewProvider(context.Background(), cfg.oidc.issuerURL)
	if err != nil {
		stdlog.Fatalf("OIDC provider initialization failed: %v", err)
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient,
		&http.Client{
			Transport: rti.NewRoundTripper("oauth", http.DefaultTransport),
		},
	)
	oidcConfig := clientcredentials.Config{
		ClientID:     cfg.oidc.clientID,
		ClientSecret: cfg.oidc.clientSecret,
		TokenURL:     provider.Endpoint().TokenURL,
	}
	if cfg.oidc.audience != "" {
		oidcConfig.EndpointParams = url.Values{
			"audience": []string{cfg.oidc.audience},
		}
	}
	client := &http.Client{
		Transport: otelhttp.NewTransport(
			&oauth2.Transport{
				Base:   rti.NewRoundTripper("ams", http.DefaultTransport),
				Source: oidcConfig.TokenSource(ctx),
			}),
	}

	if len(cfg.memcached.servers) > 0 {
		mc := memcached.New(context.Background(), cfg.memcached.interval, cfg.memcached.expire, cfg.memcached.servers...)
		client.Transport = cache.NewRoundTripper(mc, tollbooth.ExtractToken, client.Transport, log.With(logger, "component", "cache"), reg)
	}

	p := path.Join(dataEndpoint, strings.ReplaceAll(cfg.opa.pkg, ".", "/"), cfg.opa.rule)
	level.Info(logger).Log("msg", "configuring the OPA endpoint", "path", p)
	a := &authorizer{client: client, url: amsURL.String(), logger: log.With(logger, "component", "cache")}
	m := http.NewServeMux()
	m.HandleFunc(p, hi.NewHandler(prometheus.Labels{"handler": "data"}, http.HandlerFunc(newHandler(a, cfg.resourceTypePrefix, cfg.mappings))))

	if cfg.server.healthcheckURL != "" {
		// checks if server is up
		healthchecks.AddLivenessCheck("http",
			healthcheck.HTTPCheckClient(
				&http.Client{},
				cfg.server.healthcheckURL,
				http.MethodGet,
				http.StatusNotFound,
				time.Second,
			),
		)
	}

	level.Info(logger).Log("msg", "starting opa-ams")
	var g run.Group
	{
		// Signal channels must be buffered.
		sig := make(chan os.Signal, 1)
		g.Add(func() error {
			signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
			<-sig
			level.Info(logger).Log("msg", "caught interrupt")
			return nil
		}, func(_ error) {
			close(sig)
		})
	}
	{
		s := http.Server{
			Addr:    cfg.server.listen,
			Handler: otelhttp.NewHandler(m, "opa-ams", otelhttp.WithTracerProvider(tp)),
		}

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting the HTTP server", "address", cfg.server.listen)
			return s.ListenAndServe()
		}, func(err error) {
			level.Info(logger).Log("msg", "shutting down the HTTP server")
			_ = s.Shutdown(context.Background())
		})
	}
	{
		h := internalserver.NewHandler(
			internalserver.WithName("Internal - opa-ams API"),
			internalserver.WithHealthchecks(healthchecks),
			internalserver.WithPrometheusRegistry(reg),
			internalserver.WithPProf(),
		)

		s := http.Server{
			Addr:    cfg.server.listenInternal,
			Handler: otelhttp.NewHandler(h, "opa-ams-internal", otelhttp.WithTracerProvider(tp)),
		}

		g.Add(func() error {
			level.Info(logger).Log("msg", "starting internal HTTP server", "address", s.Addr)
			return s.ListenAndServe()
		}, func(err error) {
			_ = s.Shutdown(context.Background())
		})
	}

	if err := g.Run(); err != nil {
		stdlog.Fatal(err)
	}
}

func newHandler(a *authorizer, resourceTypePrefix string, mappings map[string][]string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "request must be a POST", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var req struct {
			Input struct {
				Permission string `json:"permission"`
				Resource   string `json:"resource"`
				Subject    string `json:"subject"`
				Tenant     string `json:"tenant"`
			} `json:"input"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "failed to unmarshal JSON", http.StatusInternalServerError)
			return
		}

		var action string
		switch req.Input.Permission {
		case string(rbac.Read):
			action = "get"
		case string(rbac.Write):
			action = "create"
		default:
			http.Error(w, "unknown permission", http.StatusBadRequest)
			return
		}

		allowedOrganizationIDs, ok := mappings[req.Input.Tenant]
		if !ok {
			http.Error(w, "unknown tenant", http.StatusBadRequest)
			return
		}

		resourceType := fmt.Sprintf("%s%s", strings.Title(strings.ToLower(resourceTypePrefix)), strings.Title(strings.ToLower(req.Input.Resource)))

		allowed, err := a.authorize(action, req.Input.Subject, allowedOrganizationIDs, resourceType)
		if err != nil {
			statusCode := http.StatusInternalServerError
			if sce, ok := err.(statusCoder); ok {
				statusCode = sce.statusCode()
			}
			http.Error(w, err.Error(), statusCode)
			return
		}

		w.Write([]byte(fmt.Sprintf("{\"result\":%t}", allowed)))
	}
}

type statusCoder interface {
	statusCode() int
}

type statusCodeError struct {
	error
	sc int
}

func (s *statusCodeError) statusCode() int {
	return s.sc
}

type authorizer struct {
	client *http.Client
	url    string
	logger log.Logger
}

func (a *authorizer) authorize(action string, accountUsername string, allowedOrganizationIDs []string, resourceType string) (bool, error) {
	errs := merrors.New()

	for _, orgId := range allowedOrganizationIDs {
		ar := ams.AccessReview{
			Action:          action,
			AccountUsername: accountUsername,
			OrganizationID:  orgId,
			ResourceType:    resourceType,
		}

		allowed, err := a.reviewAccessForOrgId(ar)

		if allowed {
			return true, nil
		}

		errs.Add(err)
	}

	return false, errs.Err()
}

func (a *authorizer) reviewAccessForOrgId(ar ams.AccessReview) (bool, error) {
	j, err := json.Marshal(ar)
	if err != nil {
		return false, fmt.Errorf("failed to marshal access review to JSON: %w", err)
	}

	res, err := a.client.Post(a.url, "application/json", bytes.NewBuffer(j))
	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return false, fmt.Errorf("failed to make request to AMS endpoint: %w", err)
	}

	if res.StatusCode/100 != 2 {
		msg := "got non-200 status from upstream"
		level.Error(a.logger).Log("msg", msg, "status", res.Status)
		if _, err = io.Copy(io.Discard, res.Body); err != nil {
			level.Error(a.logger).Log("msg", "failed to discard response body", "err", err.Error())
		}
		return false, &statusCodeError{errors.New(msg), res.StatusCode}
	}

	var accessReviewResponse struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(res.Body).Decode(&accessReviewResponse); err != nil {
		return false, fmt.Errorf("failed to unmarshal access review response: %w", err)
	}

	return accessReviewResponse.Allowed, nil
}

type otelErrorHandler struct {
	logger log.Logger
}

func (oh otelErrorHandler) Handle(err error) {
	level.Error(oh.logger).Log("msg", "opentelemetry", "err", err.Error())
}
