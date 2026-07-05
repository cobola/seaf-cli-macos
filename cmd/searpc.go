package cmd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

type searpcClient struct {
	conn  net.Conn
	reqID int
}

type searpcResponse struct {
	Ret     interface{} `json:"ret"`
	ErrCode int         `json:"err_code"`
	ErrMsg  string      `json:"err_msg"`
}

func newSearpcClient(socketPath string) (*searpcClient, error) {
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接 seaf-daemon 失败: %w", err)
	}
	return &searpcClient{conn: conn}, nil
}

func (c *searpcClient) call(method string, args ...interface{}) (interface{}, error) {
	c.reqID++

	// searpc 请求是 JSON 数组: [method, arg1, arg2, ...]
	innerReq := []interface{}{method}
	innerReq = append(innerReq, args...)
	innerJSON, _ := json.Marshal(innerReq)

	// named pipe 传输层包装: {"service": "...", "request": "[...]"}
	wrapReq := map[string]string{
		"service": "seafile-rpcserver",
		"request": string(innerJSON),
	}
	data, err := json.Marshal(wrapReq)
	if err != nil {
		return nil, err
	}

	// 发送: 4字节小端长度 + JSON
	length := uint32(len(data))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := c.conn.Write(lenBuf); err != nil {
		return nil, fmt.Errorf("发送长度失败: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("发送数据失败: %w", err)
	}

	// 接收: 4字节长度 + JSON
	c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	respLenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, respLenBuf); err != nil {
		return nil, fmt.Errorf("读取响应长度失败: %w", err)
	}
	respLen := binary.LittleEndian.Uint32(respLenBuf)

	respBuf := make([]byte, respLen)
	if _, err := io.ReadFull(c.conn, respBuf); err != nil {
		return nil, fmt.Errorf("读取响应数据失败: %w", err)
	}

	var resp searpcResponse
	if err := json.Unmarshal(respBuf, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w (raw: %s)", err, string(respBuf))
	}

	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("seaf-daemon 错误 (%d): %s", resp.ErrCode, resp.ErrMsg)
	}
	return resp.Ret, nil
}

func (c *searpcClient) close() {
	c.conn.Close()
}

func findSearpcSocket() string {
	// 查找 seaf-daemon 的 socket
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	// 从配置获取数据目录
	dataDir := filepath.Join(os.Getenv("HOME"), "SeafileData")
	socketPath := filepath.Join(dataDir, "seafile.sock")
	if _, err := os.Stat(socketPath); err == nil {
		return socketPath
	}

	// 尝试其他路径
	paths := []string{
		filepath.Join(dataDir, "ccnet.sock"),
		"/tmp/ccnet.sock",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// 从 seaf-daemon 进程获取
	_ = exeDir
	return ""
}
