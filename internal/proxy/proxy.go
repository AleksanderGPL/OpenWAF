package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"OpenWAF/internal/clientip"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/rules"
)

type Store interface {
	EnabledService(context.Context, string) (domain.Service, error)
}

type Service struct {
	recorder Recorder
	rules    *rules.Service
	clientIP *clientip.Resolver
	store    Store
	verified *http.Transport
	insecure *http.Transport
}

func New(store Store, recorder Recorder, clientIP *clientip.Resolver, ruleServices ...*rules.Service) *Service {
	verified := http.DefaultTransport.(*http.Transport).Clone()
	verified.Proxy = nil
	verified.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	verified.ResponseHeaderTimeout = 30 * time.Second
	insecure := verified.Clone()
	insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	s := &Service{store: store, recorder: recorder, clientIP: clientIP, verified: verified, insecure: insecure}
	if len(ruleServices) > 0 {
		s.rules = ruleServices[0]
	}
	return s
}

func (s *Service) Close() {
	s.verified.CloseIdleConnections()
	s.insecure.CloseIdleConnections()
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	ip := peer
	if s.clientIP != nil {
		ip = s.clientIP.IP(peer, r.Header.Values("X-Forwarded-For"))
	}
	event := domain.RequestLog{Timestamp: started.UTC(), Hostname: hostname(r.Host), IP: ip, Method: r.Method, Path: r.URL.EscapedPath(), Action: "allowed", Reason: "Passed all rules"}
	if event.Path == "" {
		event.Path = "/"
	}
	response := &responseWriter{ResponseWriter: w}
	w = response
	var body *requestBody
	if r.Body != nil {
		body = &requestBody{ReadCloser: r.Body}
		r.Body = body
	}
	defer func() {
		failure := recover()
		if event.ErrorCategory == "" {
			if r.Context().Err() != nil {
				event.ErrorCategory = "client_disconnect"
			} else if failure != nil {
				event.ErrorCategory = "response_stream"
			}
		}
		event.Status = response.status
		if event.Status == 0 {
			event.Status = http.StatusOK
			if failure != nil {
				event.Status = http.StatusInternalServerError
			}
		}
		event.DurationMs = float64(time.Since(started)) / float64(time.Millisecond)
		event.ResponseBytes = response.bytes
		if body != nil {
			event.RequestBytes = body.bytes.Load()
		}
		if s.recorder != nil {
			if err := s.recorder.Record(r.Context(), &event); err != nil {
				log.Printf("request log persistence failed: host=%q path=%q status=%d: %v", event.Hostname, event.Path, event.Status, err)
			}
		}
		if failure != nil {
			panic(failure)
		}
	}()
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		event.ErrorCategory = "invalid_request"
		http.Error(w, "Invalid query string", http.StatusBadRequest)
		return
	}
	if s.rules == nil {
		for _, value := range query["block"] {
			if value == "true" {
				event.Action, event.RuleID, event.Reason = "blocked", "query_block", "Query block check"
				http.Error(w, "Request blocked", http.StatusForbidden)
				return
			}
		}
	}
	service, err := s.store.EnabledService(r.Context(), event.Hostname)
	if errors.Is(err, domain.ErrServiceNotFound) {
		event.ErrorCategory = "service_not_found"
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}
	if err != nil {
		event.ErrorCategory = "service_lookup"
		log.Printf("proxy service lookup failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	event.ServiceID = &service.ID
	if s.rules != nil {
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now().Add(30 * time.Second))
		status, cleanup, inspectErr := s.rules.Inspect(r, service.ID, ip, &event)
		_ = controller.SetReadDeadline(time.Time{})
		defer cleanup()
		if inspectErr != nil {
			log.Printf("WAF processing failed for service %d", service.ID)
			if status == 0 {
				status = http.StatusInternalServerError
			}
		}
		if status != 0 {
			http.Error(w, "Request rejected by security policy", status)
			return
		}
	}
	target, err := domain.UpstreamURL(service.UpstreamURL)
	if err != nil {
		event.ErrorCategory = "upstream_configuration"
		log.Printf("invalid upstream for service %d: %v", service.ID, err)
		http.Error(w, "Invalid upstream configuration", http.StatusBadGateway)
		return
	}
	transport := s.verified
	if service.SkipTLSVerify {
		transport = s.insecure
	}
	proxy := &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(target)
			request.Out.Host = request.In.Host
			request.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			event.ErrorCategory = "upstream_unavailable"
			log.Printf("proxy request to service %d failed: %v", service.ID, err)
			http.Error(w, "Upstream unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
