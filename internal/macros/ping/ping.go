// Package ping implements a single-node HTTP ping via a Vendor proxy.
package ping

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptrace"
	urllib "net/url"
	"strings"
	"time"

	"github.com/openpaily/paily-check/internal/interfaces"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// PingConfig carries parameters for a single ping run.
type PingConfig struct {
	URL       string
	Avg       int
	TimeoutMS int
}

// PingResult is the outcome of Ping().
type PingResult struct {
	LatencyMS int // -1 = all attempts timed out
	JitterMS  int // population stddev over N samples (0 for initial pass if not needed)
}

// Ping performs up to cfg.Avg attempts against cfg.URL through vendor v.
// It calculates latency and jitter.
//
// Average rule (N = cfg.Avg, T = cfg.TimeoutMS):
//
//	if all N attempts fail → LatencyMS = -1
//	otherwise             → sum = Σ(success_rtts) + failed * T
//	                        LatencyMS = sum / N
//
// JitterMS = population stddev across all N sample values (failures → T).
func Ping(ctx context.Context, v interfaces.Vendor, cfg PingConfig) PingResult {
	n := cfg.Avg
	if n <= 0 {
		n = 3
	}
	t := cfg.TimeoutMS
	if t <= 0 {
		t = 3000
	}

	samples := make([]int, n) // filled with T or actual rtt
	successCount := 0

	for i := 0; i < n; i++ {
		httpDelay, err := singlePing(ctx, v, cfg.URL, time.Duration(t)*time.Millisecond)
		if err != nil {
			samples[i] = t
		} else {
			samples[i] = int(httpDelay)
			successCount++
		}
		time.Sleep(1 * time.Second)
	}

	if successCount == 0 {
		return PingResult{LatencyMS: -1, JitterMS: -1}
	}

	// average = (Σ samples) / N
	sum := 0
	for _, s := range samples {
		sum += s
	}
	avg := sum / n

	// jitter = population stddev over all N samples
	variance := 0.0
	favg := float64(sum) / float64(n)
	for _, s := range samples {
		d := float64(s) - favg
		variance += d * d
	}
	variance /= float64(n)
	jitter := int(math.Round(math.Sqrt(variance)))

	return PingResult{LatencyMS: avg, JitterMS: jitter}
}

// ---------------------------------------------------------------------------
// Low-level: single attempt
// ---------------------------------------------------------------------------

func singlePing(ctx context.Context, v interfaces.Vendor, rawURL string, timeout time.Duration) (httpDelay int64, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if strings.HasPrefix(rawURL, "https:") {
		_, http, _, e := pingViaTrace(ctx, v, rawURL)
		return int64(http), e
	}
	_, http, _, e := pingViaNetCat(ctx, v, rawURL)
	return int64(http), e
}

// ---------------------------------------------------------------------------
// Ping via HTTPS trace (TLS round-trip measurement)
// ---------------------------------------------------------------------------

func pingViaTrace(ctx context.Context, p interfaces.Vendor, url string) (rtt, req uint16, statusCode int, err error) {
	transport := &http.Transport{
		DialContext: func(dCtx context.Context, _, _ string) (net.Conn, error) {
			return p.DialTCP(dCtx, url, interfaces.ROptionsTCP)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       3 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			// TLS 1.2 minimum: most production endpoints (including the default
			// gstatic.com/generate_204 ping URL) support 1.2; requiring 1.3
			// would silently fail against valid TLS-1.2-only targets.
			MinVersion: tls.VersionTLS12,
		},
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, 0, err
	}

	var tlsEnd, writeStart, writeEnd int64
	trace := &httptrace.ClientTrace{
		TLSHandshakeDone:     func(_ tls.ConnectionState, _ error) { tlsEnd = time.Now().UnixMilli() },
		GotFirstResponseByte: func() { writeEnd = time.Now().UnixMilli() },
		WroteHeaders:         func() { writeStart = time.Now().UnixMilli() },
	}
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(ctx, trace))
	_ = writeStart // kept for reference; unused in return calculation

	start := time.Now().UnixMilli()
	resp, err := transport.RoundTrip(httpReq)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	connEnd := time.Now().UnixMilli()

	if !strings.HasPrefix(url, "https:") {
		return uint16(connEnd - start), uint16(connEnd - start), resp.StatusCode, nil
	}
	if resp.TLS != nil && resp.TLS.HandshakeComplete {
		return uint16(writeEnd - tlsEnd), uint16(writeEnd - start), resp.StatusCode, nil
	}
	return 0, 0, 0, fmt.Errorf("TLS handshake did not complete")
}

// ---------------------------------------------------------------------------
// Ping via plain HTTP netcat (TCP round-trip measurement)
// ---------------------------------------------------------------------------

const netcatPayload = "GET %s HTTP/1.1\r\nAccept: */*\r\nAccept-Encoding: gzip, deflate\r\nHost: %s\r\nUser-Agent: HTTPie/3.0.2 PailyCheck/1.0\r\n\r\n"

type timeoutReader struct {
	r       *bufio.Reader
	timeout time.Time
}

func (tr *timeoutReader) Read(p []byte) (n int, err error) {
	if time.Now().After(tr.timeout) {
		return 0, errors.New("read timeout")
	}
	return tr.r.Read(p)
}

func saferParseHTTPStatus(reader *bufio.Reader) (int, error) {
	tr := &timeoutReader{reader, time.Now().Add(5 * time.Second)}
	lr := io.LimitReader(tr, 1024*1024)
	resp, err := http.ReadResponse(bufio.NewReader(lr), nil)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

func pingViaNetCat(ctx context.Context, p interfaces.Vendor, rawURL string) (rtt, req uint16, statusCode int, err error) {
	purl, parseErr := urllib.Parse(rawURL)
	if parseErr != nil {
		return 0, 0, 0, parseErr
	}
	path := purl.EscapedPath()
	if purl.RawQuery != "" {
		path += "?" + purl.Query().Encode()
	}
	if path == "" {
		path = "/"
	}
	payload := fmt.Sprintf(netcatPayload, path, purl.Hostname())

	connStart := time.Now()
	conn, err := p.DialTCP(ctx, rawURL, interfaces.ROptionsTCP)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	reader := bufio.NewReader(conn)

	tcpStart := time.Now()
	if _, err := conn.Write([]byte(payload)); err != nil {
		return 0, 0, 0, fmt.Errorf("write 1: %w", err)
	}
	if _, err := reader.Peek(1); err != nil {
		return 0, 0, 0, fmt.Errorf("peek 1: %w", err)
	}

	tcpRTT := time.Since(tcpStart).Milliseconds()
	connRTT := time.Since(connStart).Milliseconds()
	statusCode, _ = saferParseHTTPStatus(reader)
	for reader.Buffered() > 0 {
		_, _, _ = reader.ReadLine()
	}

	// second request for a cleaner RTT sample
	tcpStart = time.Now()
	if _, err := conn.Write([]byte(payload)); err != nil {
		return uint16(tcpRTT), uint16(connRTT), statusCode, nil
	}
	if _, err := reader.Peek(1); err != nil {
		if err == io.EOF {
			return uint16(tcpRTT), uint16(connRTT), statusCode, nil
		}
		return uint16(tcpRTT), uint16(connRTT), statusCode, nil
	}
	tcpRTT = time.Since(tcpStart).Milliseconds()
	statusCode2, err2 := saferParseHTTPStatus(reader)
	if err2 == nil {
		statusCode = statusCode2
	}

	return uint16(tcpRTT), uint16(connRTT), statusCode, nil
}
