package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"os"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/observatorium/opa-ams/ams"
)

func main() {
	listen := flag.String("web.listen", ":8080", "The address on which the public server listens.")
	issuerURL := flag.String("oidc.issuer-url", "", "The OIDC issuer URL, see https://openid.net/specs/openid-connect-discovery-1_0.html#IssuerDiscovery.")
	clientID := flag.String("oidc.client-id", "", "The OIDC client ID, see https://tools.ietf.org/html/rfc6749#section-2.3.")
	path := flag.String("ams.access-reviews", "", "A path to a JSON file containing valid access reviews.")
	flag.Parse()

	if len(*path) == 0 {
		stdlog.Fatalf("--ams.access-reviews is required")
	}

	data, err := ioutil.ReadFile(*path)
	if err != nil {
		stdlog.Fatalf("unable to read JSON file: %v", err)
	}

	var valid []map[string]string
	if err := json.Unmarshal(data, &valid); err != nil {
		stdlog.Fatalf("unable to parse contents of %s: %v", *path, err)
	}

	provider, err := oidc.NewProvider(context.Background(), *issuerURL)
	if err != nil {
		stdlog.Fatalf("OIDC provider initialization failed: %v", err)
	}

	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	l = log.WithPrefix(l, "ts", log.DefaultTimestampUTC)
	l = log.WithPrefix(l, "caller", log.DefaultCaller)
	level.Info(l).Log("msg", "mock-ams initialized")

	verifier := provider.Verifier(&oidc.Config{ClientID: *clientID})
	m := http.NewServeMux()
	m.HandleFunc(ams.AccessReviewEndpoint, newHandler(valid, verifier))

	s := http.Server{
		Addr:    *listen,
		Handler: m,
	}

	level.Info(l).Log("msg", "starting the HTTP server", "address", *listen)
	s.ListenAndServe()
}

func newHandler(valid []map[string]string, verifier *oidc.IDTokenVerifier) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := strings.Split(r.Header.Get("Authorization"), " ")
		if len(authorization) != 2 {
			http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
			return
		}

		if _, err := verifier.Verify(oidc.ClientContext(r.Context(), http.DefaultClient), authorization[1]); err != nil {
			http.Error(w, "failed to authenticate", http.StatusBadRequest)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		req := make(map[string]string)
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "failed to unmarshal JSON", http.StatusInternalServerError)
			return
		}

		var ok bool
		for i := range valid {
			for k, v := range valid[i] {
				if req[k] == v {
					ok = true
					continue
				}

				ok = false

				break
			}

			if ok {
				break
			}
		}
		w.Write([]byte(fmt.Sprintf("{\"allowed\":%t}", ok)))
	}
}
