package llm

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPPool is a high-performance HTTP client connection pool manager
type HTTPPool struct {
	mu      sync.RWMutex
	clients map[string]*http.Client
	config  HTTPPoolConfig
}

// HTTPPoolConfig HTTP connection pool configuration
type HTTPPoolConfig struct {
	MaxIdleConns          int           // Maximum idle connections
	MaxIdleConnsPerHost   int           // Maximum idle connections per host
	IdleConnTimeout       time.Duration // Idle connection timeout
	TLSHandshakeTimeout   time.Duration // TLS handshake timeout
	ResponseHeaderTimeout time.Duration // Response header timeout
	RequestTimeout        time.Duration // Total request timeout
	MaxRetries            int           // Maximum retry count
}

// DefaultHTTPPoolConfig returns default HTTP pool configuration
func DefaultHTTPPoolConfig() HTTPPoolConfig {
	return HTTPPoolConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		RequestTimeout:        30 * time.Second,
		MaxRetries:            3,
	}
}

var (
	defaultPool     *HTTPPool
	defaultPoolOnce sync.Once
)

// GetDefaultPool 獲取默認的 HTTP 連接池
func GetDefaultPool() *HTTPPool {
	defaultPoolOnce.Do(func() {
		defaultPool = NewHTTPPool(DefaultHTTPPoolConfig())
	})
	return defaultPool
}

// NewHTTPPool 創建新的 HTTP 連接池
func NewHTTPPool(config HTTPPoolConfig) *HTTPPool {
	return &HTTPPool{
		clients: make(map[string]*http.Client),
		config:  config,
	}
}

// GetClient 獲取或創建 HTTP 客戶端
// key 用於區分不同的配置，例如不同的 CA 證書或 TLS 設置
func (p *HTTPPool) GetClient(key string, tlsConfig *tls.Config) *http.Client {
	p.mu.RLock()
	if client, exists := p.clients[key]; exists {
		p.mu.RUnlock()
		return client
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 雙重檢查鎖定模式
	if client, exists := p.clients[key]; exists {
		return client
	}

	// 創建優化的傳輸層
	transport := &http.Transport{
		MaxIdleConns:          p.config.MaxIdleConns,
		MaxIdleConnsPerHost:   p.config.MaxIdleConnsPerHost,
		IdleConnTimeout:       p.config.IdleConnTimeout,
		TLSHandshakeTimeout:   p.config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: p.config.ResponseHeaderTimeout,

		// 啟用 HTTP/2
		ForceAttemptHTTP2: true,

		// 使用系統代理設置
		Proxy: http.ProxyFromEnvironment,

		// 自定義 TLS 配置
		TLSClientConfig: tlsConfig,

		// 連接超時設置
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   p.config.RequestTimeout,
	}

	p.clients[key] = client
	return client
}

// GetOptimizedClient 獲取針對特定用途優化的客戶端
func (p *HTTPPool) GetOptimizedClient(purpose ClientPurpose, tlsConfig *tls.Config) *http.Client {
	var key string
	var timeout time.Duration

	switch purpose {
	case ClientPurposeAPI:
		key = "api_client"
		timeout = 30 * time.Second
	case ClientPurposeStreaming:
		key = "streaming_client"
		timeout = 5 * time.Minute // 更長的超時用於流式響應
	case ClientPurposeHealthCheck:
		key = "health_client"
		timeout = 5 * time.Second // 更短的超時用於健康檢查
	default:
		key = "default_client"
		timeout = p.config.RequestTimeout
	}

	p.mu.RLock()
	if client, exists := p.clients[key]; exists {
		p.mu.RUnlock()
		return client
	}
	p.mu.RUnlock()

	client := p.GetClient(key, tlsConfig)
	client.Timeout = timeout
	return client
}

// ClientPurpose 定義客戶端用途
type ClientPurpose string

const (
	ClientPurposeAPI         ClientPurpose = "api"
	ClientPurposeStreaming   ClientPurpose = "streaming"
	ClientPurposeHealthCheck ClientPurpose = "health"
)

// Close 關閉連接池中的所有客戶端
func (p *HTTPPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, client := range p.clients {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	p.clients = make(map[string]*http.Client)
}

// Stats 返回連接池統計信息
type PoolStats struct {
	ActiveClients int
	TotalRequests int64
}

// GetStats 獲取連接池統計信息
func (p *HTTPPool) GetStats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStats{
		ActiveClients: len(p.clients),
	}
}
