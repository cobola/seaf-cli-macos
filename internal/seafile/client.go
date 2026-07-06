package seafile

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client 海量文件 API 客户端
type Client struct {
	server string
	token  string
	http   *http.Client
}

// NewClient 创建 API 客户端，复用 HTTP 连接
func NewClient(server, token string) *Client {
	return &Client{
		server: strings.TrimRight(server, "/"),
		token:  token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

// Get 发送 GET 请求
func (c *Client) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.server+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	return c.http.Do(req)
}

// Post 发送 POST 请求
func (c *Client) Post(path, body string) (*http.Response, error) {
	req, err := http.NewRequest("POST", c.server+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.http.Do(req)
}

// IsWafBlocked 检查是否被 WAF 封禁
func IsWafBlocked(statusCode int, body string) bool {
	return statusCode == 405 || statusCode == 429 ||
		strings.Contains(body, "blocked") || strings.Contains(body, "WAF")
}

// WaitForWaf 等待 WAF 解封
func WaitForWaf(attempt int) time.Duration {
	// 指数退避：30s, 60s, 120s
	delay := time.Duration(30*(1<<uint(attempt))) * time.Second
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return delay
}

// ValidateURL 校验 URL
func ValidateURL(server string) error {
	if server == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return fmt.Errorf("服务器地址必须以 http:// 或 https:// 开头")
	}
	return nil
}
