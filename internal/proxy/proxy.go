package proxy

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"OpenWAF/internal/database"
	"gorm.io/gorm"
)

type Service struct {
	db       *gorm.DB
	verified *http.Transport
	insecure *http.Transport
}

func New(db *gorm.DB) *Service {
	verified := http.DefaultTransport.(*http.Transport).Clone()
	verified.Proxy = nil
	verified.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	verified.ResponseHeaderTimeout = 30 * time.Second
	insecure := verified.Clone()
	insecure.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Service{db: db, verified: verified, insecure: insecure}
}

func (s *Service) Close() {
	s.verified.CloseIdleConnections()
	s.insecure.CloseIdleConnections()
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Invalid query string", http.StatusBadRequest)
		return
	}
	for _, value := range query["block"] {
		if value == "true" {
			http.Error(w, "Request blocked", http.StatusForbidden)
			return
		}
	}
	host := r.Host
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	var service database.Service
	if err := s.db.WithContext(r.Context()).Where("hostname = ? AND enabled = ?", host, true).Limit(1).Find(&service).Error; err != nil {
		log.Printf("proxy service lookup failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if service.ID == 0 {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}
	target, err := upstreamURL(service.UpstreamURL)
	if err != nil {
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
			log.Printf("proxy request to service %d failed: %v", service.ID, err)
			http.Error(w, "Upstream unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
