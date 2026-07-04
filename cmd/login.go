package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/your-username/seaf-cli-macos/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "登录 Seafile 服务器",
	Long: `登录方式：
  --web                弹出网页登录（支持密码登录 / SSO 扫码）
  --config <file.json> 直接导入 JSON 配置`,
	RunE: runLogin,
}

var (
	webMode    bool
	configFile string
	initRootDir string
)

func init() {
	loginCmd.Flags().BoolVar(&webMode, "web", false, "弹出网页登录界面")
	loginCmd.Flags().StringVar(&configFile, "config", "", "JSON 配置文件路径（含 server/email/token 字段）")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	rootDir := initRootDir
	if rootDir == "" {
		rootDir = config.GetDefaultRootDir()
	}

	switch {
	case configFile != "":
		return loginWithConfig(rootDir)
	case webMode:
		return loginWithWeb(rootDir)
	default:
		return cmd.Help()
	}
}

// --config 模式：直接读取 JSON 文件导入
func loginWithConfig(rootDir string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}
	var cred struct {
		Server string `json:"server"`
		Email  string `json:"email"`
		Token  string `json:"token"`
	}
	if err := json.Unmarshal(data, &cred); err != nil {
		return fmt.Errorf("JSON 解析失败: %w\n格式示例: {\"server\":\"https://x.com\",\"email\":\"a@b.com\",\"token\":\"xxx\"}", err)
	}
	if cred.Server == "" || cred.Token == "" {
		return fmt.Errorf("server 和 token 字段必填")
	}
	server := normalizeServerURL(cred.Server)
	if err := validateToken(server, cred.Token); err != nil {
		return fmt.Errorf("Token 验证失败: %w", err)
	}
	cfg := config.NewConfig(rootDir)
	if err := cfg.Load(); err != nil {
		return err
	}
	cfg.Server = server
	cfg.Username = cred.Email
	cfg.Token = cred.Token
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Printf("✓ 登录成功  服务器: %s\n", server)
	if cred.Email != "" {
		fmt.Printf("  用户: %s\n", cred.Email)
	}
	return nil
}

// --web 模式：本地 HTTP 页面 + 后端接口
func loginWithWeb(rootDir string) error {
	type loginResult struct{ server, email, token string }
	resultCh := make(chan loginResult, 1)

	mux := http.NewServeMux()

	// 首页：登录表单
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, loginHTML)
	})

	// API：密码登录 → Seafile api2/auth-token
	mux.HandleFunc("/api/password-login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		server := r.FormValue("server")
		username := r.FormValue("username")
		password := r.FormValue("password")
		if server == "" || username == "" || password == "" {
			jsonError(w, "服务器地址、用户名和密码不能为空")
			return
		}
		server = normalizeServerURL(server)

		// 调用 Seafile API 获取 token
		token, err := fetchAuthToken(server, username, password)
		if err != nil {
			jsonError(w, err.Error())
			return
		}

		// 保存配置
		cfg := config.NewConfig(rootDir)
		if err := cfg.Load(); err != nil {
			jsonError(w, err.Error())
			return
		}
		cfg.Server = server
		cfg.Username = username
		cfg.Token = token
		if err := cfg.Save(); err != nil {
			jsonError(w, err.Error())
			return
		}

		select {
		case resultCh <- loginResult{server: server, email: username, token: token}:
		default:
		}
		jsonOK(w, fmt.Sprintf("登录成功: %s", server))
	})

	// API：Token 登录
	mux.HandleFunc("/api/token-login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		server := r.FormValue("server")
		email := r.FormValue("email")
		token := r.FormValue("token")
		if server == "" || token == "" {
			jsonError(w, "服务器地址和 Token 不能为空")
			return
		}
		server = normalizeServerURL(server)
		if err := validateToken(server, token); err != nil {
			jsonError(w, err.Error())
			return
		}
		cfg := config.NewConfig(rootDir)
		if err := cfg.Load(); err != nil {
			jsonError(w, err.Error())
			return
		}
		cfg.Server = server
		cfg.Username = email
		cfg.Token = token
		if err := cfg.Save(); err != nil {
			jsonError(w, err.Error())
			return
		}
		select {
		case resultCh <- loginResult{server: server, email: email, token: token}:
		default:
		}
		jsonOK(w, fmt.Sprintf("登录成功: %s", server))
	})

	// API：检测服务器可达性
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		server := normalizeServerURL(r.URL.Query().Get("server"))
		if server == "" {
			jsonError(w, "参数缺失")
			return
		}
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(server + "/api2/auth/ping/")
		if err != nil {
			jsonError(w, fmt.Sprintf("服务器不可达: %s", err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			jsonOK(w, "服务器连接正常")
		} else {
			jsonError(w, fmt.Sprintf("服务器响应: HTTP %d", resp.StatusCode))
		}
	})

	ln, err := openPort()
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: mux, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second}
	go srv.Serve(ln)
	defer srv.Shutdown(nil)

	addr := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf("正在打开登录页面...\n")
	exec.Command("open", addr).Start()

	select {
	case r := <-resultCh:
		fmt.Printf("\n✓ 登录成功\n  服务器: %s\n", r.server)
		if r.email != "" {
			fmt.Printf("  用户: %s\n", r.email)
		}
		return nil
	case <-time.After(15 * time.Minute):
		return fmt.Errorf("登录超时")
	}
}

func fetchAuthToken(server, username, password string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	form := url.Values{"username": {username}, "password": {password}}
	resp, err := client.Post(server+"/api2/auth-token/", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("连接服务器失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		// 尝试解析 Seafile 错误信息
		var seaErr struct {
			ErrorMsg string `json:"error_msg"`
			Detail   string `json:"detail"`
		}
		if json.Unmarshal(body, &seaErr) == nil && seaErr.ErrorMsg != "" {
			return "", fmt.Errorf("%s", seaErr.ErrorMsg)
		}
		if seaErr.Detail != "" {
			return "", fmt.Errorf("%s", seaErr.Detail)
		}
		return "", fmt.Errorf("登录失败 (HTTP %d)", resp.StatusCode)
	}
	var result struct {
		Token string `json:"token"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	token := result.Token
	if token == "" {
		token = result.Key
	}
	if token == "" {
		return "", fmt.Errorf("服务器未返回 Token")
	}
	return token, nil
}

func openPort() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:8765")
}

func validateToken(server, token string) error {
	req, err := http.NewRequest("GET", server+"/api2/auth/ping/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Token 无效 (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func normalizeServerURL(s string) string {
	s = strings.TrimRight(s, "/")
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return s
}

func jsonOK(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"ok":true,"message":"%s"}`, msg)
}

func jsonError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]string{"ok": "false", "error": msg})
	w.Write(b)
}

// --- HTML ---

const loginHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Seafile 客户端登录</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"PingFang SC","Microsoft YaHei","Helvetica Neue",sans-serif;background:#f0f2f5;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.box{background:#fff;border-radius:12px;box-shadow:0 2px 12px rgba(0,0,0,.1);width:100%;max-width:460px;overflow:hidden}
.header{padding:28px 32px 12px}
.header h1{font-size:20px;color:#ff8800;font-weight:700}
.header .sub{font-size:13px;color:#999;margin-top:4px}
.form{padding:0 32px 28px}
.field{margin-bottom:16px}
.field label{display:block;font-size:13px;color:#666;margin-bottom:6px}
.field input{width:100%;padding:10px 12px;border:1px solid #d9d9d9;border-radius:6px;font-size:14px;outline:none;transition:border-color .2s}
.field input:focus{border-color:#ff8800}
.field .hint{font-size:11px;color:#bbb;margin-top:4px}
.login-btn{width:100%;padding:12px;background:#ff8800;color:#fff;border:none;border-radius:6px;font-size:15px;font-weight:600;cursor:pointer;transition:opacity .2s}
.login-btn:hover{opacity:.9}
.login-btn:disabled{background:#ccc;cursor:not-allowed}
.sso-link{display:block;text-align:left;margin-top:16px;font-size:13px;color:#1677ff;cursor:pointer;text-decoration:none}
.sso-link:hover{text-decoration:underline}
.note{margin-top:12px;font-size:12px;color:#999;line-height:1.6}
.note code{background:#f5f5f5;padding:2px 6px;border-radius:3px;font-size:11px}
#status{margin-top:12px;padding:10px;border-radius:6px;font-size:13px;line-height:1.5;display:none}
.s-ok{background:#f6ffed;color:#52c41a;border:1px solid #b7eb8f;display:block}
.s-err{background:#fff2f0;color:#ff4d4f;border:1px solid #ffccc7;display:block}
.s-load{background:#e6f7ff;color:#1677ff;border:1px solid #91d5ff;display:block}
.divider{border:none;border-top:1px solid #f0f0f0;margin:20px 0}
.manual-section{margin-top:8px}
.manual-toggle{font-size:13px;color:#999;cursor:pointer;user-select:none}
.manual-toggle:hover{color:#666}
.manual-form{display:none;margin-top:12px}
.manual-form .field{margin-bottom:12px}
.manual-form button{width:100%;padding:10px;background:#ff8800;color:#fff;border:none;border-radius:6px;font-size:14px;cursor:pointer}
.manual-form button:disabled{background:#ccc}
</style>
</head>
<body>
<div class="box">
  <div class="header">
    <h1>添加帐号</h1>
    <div class="sub">连接到 Seafile 服务器</div>
  </div>
  <div class="form">
    <!-- 主登录表单 -->
    <div class="field">
      <label>服务器地址</label>
      <input id="server" placeholder="例如：https://seacloud.cc 或 http://192.168.1.24:8000">
      <div class="hint">输入 Seafile 服务器 URL</div>
    </div>
    <div class="field">
      <label>邮箱 / 用户名</label>
      <input id="username" placeholder="your@email.com">
    </div>
    <div class="field">
      <label>密码</label>
      <input id="password" type="password" placeholder="密码">
    </div>
    <button class="login-btn" id="loginBtn" onclick="doLogin()">登录</button>

    <a class="sso-link" onclick="toggleSSO()">单点登录或 Token 登录</a>

    <!-- SSO / 手动 Token 展开区 -->
    <div id="ssoSection" style="display:none">
      <hr class="divider">
      <div class="manual-section">
        <div class="manual-toggle" onclick="toggleManual()">▶ 手动输入 API Token（跳过网页登录）</div>
        <div class="manual-form" id="manualForm">
          <div class="field">
            <label>API Token</label>
            <input id="manualToken" placeholder="粘贴从网页设置中获取的 API Token">
          </div>
          <div class="field">
            <label>用户（可选）</label>
            <input id="manualEmail" placeholder="your@email.com">
          </div>
          <button id="manualBtn" onclick="doManualToken()">验证并登录</button>
        </div>
      </div>
      <div class="note">
        <strong>Token 获取方式：</strong>登录服务器网页 → 个人设置 → API 令牌 → 生成新令牌，复制粘贴到上方输入框
      </div>
    </div>

    <div id="status"></div>

    <div class="note" style="margin-top:16px">
      也可通过命令行直接导入：<br>
      <code>seaf-cli login --config config.json</code><br>
      配置文件格式：{"server":"...","email":"...","token":"..."}
    </div>
  </div>
</div>

<script>
function toggleSSO(){
  var s=document.getElementById('ssoSection');
  s.style.display=s.style.display==='none'?'block':'none';
}
function toggleManual(){
  var f=document.getElementById('manualForm');
  var t=document.querySelector('.manual-toggle');
  var show=f.style.display==='none';
  f.style.display=show?'block':'none';
  t.textContent=(show?'▼':'▶')+' 手动输入 API Token（跳过网页登录）';
}
function setStatus(id,cls,txt){
  var el=document.getElementById(id);
  el.className='s-'+cls;el.textContent=txt;el.style.display='block';
}
function normalize(s){
  s=s.trim().replace(/\/+$/,'');
  if(!/^https?:\/\//.test(s)) s='https://'+s;
  return s;
}

async function doLogin(){
  var server=normalize(document.getElementById('server').value);
  var username=document.getElementById('username').value.trim();
  var password=document.getElementById('password').value;
  var st=document.getElementById('status');
  var btn=document.getElementById('loginBtn');

  if(!server||!username||!password){setStatus(st,'err','请填写完整信息');return}
  btn.disabled=true;btn.textContent='登录中...';
  setStatus(st,'load','正在连接服务器...');

  try{
    var body=new URLSearchParams({server:server,username:username,password:password});
    var r=await fetch('/api/password-login',{method:'POST',body:body});
    var d=await r.json();
    if(d.ok==='true'||d.ok===true){
      setStatus(st,'ok',d.message||'登录成功！可关闭此页面');
    }else{
      throw new Error(d.error||'登录失败');
    }
  }catch(e){
    setStatus(st,'err',e.message);
  }
  btn.disabled=false;btn.textContent='登录';
}

async function doManualToken(){
  var server=normalize(document.getElementById('server').value);
  var token=document.getElementById('manualToken').value.trim();
  var email=document.getElementById('manualEmail').value.trim();
  var st=document.getElementById('status');
  var btn=document.getElementById('manualBtn');

  if(!server||!token){setStatus(st,'err','请填写服务器地址和 Token');return}
  btn.disabled=true;btn.textContent='验证中...';
  setStatus(st,'load','正在验证 Token...');

  try{
    var body=new URLSearchParams({server:server,email:email,token:token});
    var r=await fetch('/api/token-login',{method:'POST',body:body});
    var d=await r.json();
    if(d.ok==='true'||d.ok===true){
      setStatus(st,'ok',d.message||'登录成功！可关闭此页面');
    }else{
      throw new Error(d.error||'验证失败');
    }
  }catch(e){
    setStatus(st,'err',e.message);
  }
  btn.disabled=false;btn.textContent='验证并登录';
}

// Enter 键提交
document.getElementById('password').addEventListener('keydown',function(e){if(e.key==='Enter')doLogin()});
document.getElementById('manualToken').addEventListener('keydown',function(e){if(e.key==='Enter')doManualToken()});
</script>
</body>
</html>`
