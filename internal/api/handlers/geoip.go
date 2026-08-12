package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/autobrr/autobrr/pkg/ttlcache"
	"github.com/rs/zerolog/log"
)

type GeoIPResult struct {
	Isp string `json:"isp"`
}

type GeoIPHandler struct {
	client  *http.Client
	cache   *ttlcache.Cache[string, *GeoIPResult]
	limiter *tokenBucket
}

func NewGeoIPHandler() *GeoIPHandler {
	return &GeoIPHandler{
		client:  &http.Client{Timeout: 10 * time.Second},
		cache:   ttlcache.New(ttlcache.Options[string, *GeoIPResult]{}.SetDefaultTTL(24 * time.Hour)),
		limiter: newTokenBucket(100, 100),
	}
}

type geoIPRequest struct {
	Ips []string `json:"ips"`
}

type geoIPResponse struct {
	Results map[string]*string `json:"results"`
}

func (h *GeoIPHandler) HandleBatchLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req geoIPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Ips) == 0 {
		http.Error(w, "no IPs provided", http.StatusBadRequest)
		return
	}

	results := make(map[string]*string, len(req.Ips))

	for _, ip := range req.Ips {
		if ip == "" {
			continue
		}

		// Check cache first
		if cached, ok := h.cache.Get(ip); ok {
			results[ip] = &cached.Isp
			continue
		}

		// Rate limit before querying
		h.limiter.Wait()

		result, err := h.lookupISP(r.Context(), ip)
		if err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("GeoIP lookup failed")
			results[ip] = nil
			continue
		}

		h.cache.Set(ip, result, 24*time.Hour)
		results[ip] = &result.Isp
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(geoIPResponse{Results: results})
}

func (h *GeoIPHandler) lookupISP(ctx context.Context, ip string) (*GeoIPResult, error) {
	url := fmt.Sprintf("https://hackmyip.com/api/asn?q=%s", ip)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "qui/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Isp string `json:"isp"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !result.Success || result.Data.Isp == "" {
		return nil, fmt.Errorf("empty ISP in response")
	}

	return &GeoIPResult{Isp: result.Data.Isp}, nil
}

// tokenBucket implements a simple rate limiter.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func newTokenBucket(maxTokens, refillPerSec float64) *tokenBucket {
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillPerSec,
		lastRefill: time.Now(),
	}
}

func (tb *tokenBucket) Wait() {
	tb.mu.Lock()
	now := time.Now()

	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
	tb.lastRefill = now

	if tb.tokens < 1 {
		need := time.Duration((1 - tb.tokens) / tb.refillRate * float64(time.Second))
		tb.mu.Unlock()
		time.Sleep(need)
		tb.mu.Lock()
		tb.tokens = 0
		now = time.Now()
		elapsed = now.Sub(tb.lastRefill).Seconds()
		tb.tokens = min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
		tb.lastRefill = now
	}

	tb.tokens--
	tb.mu.Unlock()
}
