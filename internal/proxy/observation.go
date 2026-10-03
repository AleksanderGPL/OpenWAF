package proxy

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"OpenWAF/internal/domain"
)

type Recorder interface {
	Record(context.Context, *domain.RequestLog) error
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	if status >= 200 || status == http.StatusSwitchingProtocols {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += int64(n)
	return n, err
}
func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil && w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type requestBody struct {
	io.ReadCloser
	bytes atomic.Int64
}

func (b *requestBody) Read(buffer []byte) (int, error) {
	n, err := b.ReadCloser.Read(buffer)
	b.bytes.Add(int64(n))
	return n, err
}

func hostname(host string) string {
	if value, _, err := net.SplitHostPort(host); err == nil {
		host = value
	}
	return strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
}
