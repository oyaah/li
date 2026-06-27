package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultStartURL = "https://www.linkedin.com/feed/"

type Options struct {
	ChromePath       string
	UserDataDir      string
	ProfileDirectory string
	StartURL         string
	StartupTimeout   time.Duration
}

type Browser struct {
	Port        int
	UserDataDir string
	cmd         *exec.Cmd
	page        *cdpConn
}

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path,omitempty"`
	URL    string `json:"url,omitempty"`
}

func DefaultUserDataDir() string {
	if base, err := os.UserConfigDir(); err == nil && base != "" {
		return filepath.Join(base, "li", "chrome-profile")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".li", "chrome-profile")
	}
	return filepath.Join(os.TempDir(), "li-chrome-profile")
}

func DefaultChromeUserDataDir() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library/Application Support/Google/Chrome")
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Google/Chrome/User Data")
	default:
		return filepath.Join(home, ".config/google-chrome")
	}
}

func FindChrome(path string) (string, error) {
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("Chrome executable not found at %s", path)
	}
	if env := os.Getenv("LI_CHROME"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env, nil
		}
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		}
	case "windows":
		candidates = []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), "Google/Chrome/Application/chrome.exe"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google/Chrome/Application/chrome.exe"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Google/Chrome/Application/chrome.exe"),
		}
	default:
		candidates = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, string(os.PathSeparator)) {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if p, err := exec.LookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", errors.New("Google Chrome not found; install Chrome or set LI_CHROME")
}

func Launch(ctx context.Context, opts Options) (*Browser, error) {
	chrome, err := FindChrome(opts.ChromePath)
	if err != nil {
		return nil, err
	}
	if opts.UserDataDir == "" {
		opts.UserDataDir = DefaultUserDataDir()
	}
	if opts.StartURL == "" {
		opts.StartURL = defaultStartURL
	}
	if opts.StartupTimeout <= 0 {
		opts.StartupTimeout = 30 * time.Second
	}
	if err := os.MkdirAll(opts.UserDataDir, 0o700); err != nil {
		return nil, err
	}
	if opts.ProfileDirectory == "" {
		opts.ProfileDirectory = DetectProfileDirectory(opts.UserDataDir)
	}
	if port, err := readDevToolsPort(opts.UserDataDir); err == nil {
		if ws, err := openPage(ctx, port, opts.StartURL); err == nil {
			return &Browser{Port: port, UserDataDir: opts.UserDataDir, page: &cdpConn{ws: ws}}, nil
		}
	}
	_ = os.Remove(filepath.Join(opts.UserDataDir, "DevToolsActivePort"))
	args := []string{
		"--remote-debugging-port=0",
		"--user-data-dir=" + opts.UserDataDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--new-window",
		"about:blank",
	}
	if opts.ProfileDirectory != "" {
		args = append(args[:2], append([]string{"--profile-directory=" + opts.ProfileDirectory}, args[2:]...)...)
	}
	cmd := exec.CommandContext(ctx, chrome, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	port, err := waitDevToolsPort(ctx, opts.UserDataDir, opts.StartupTimeout)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	ws, err := openPage(ctx, port, opts.StartURL)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	return &Browser{Port: port, UserDataDir: opts.UserDataDir, cmd: cmd, page: &cdpConn{ws: ws}}, nil
}

func DetectProfileDirectory(userDataDir string) string {
	matches, err := filepath.Glob(filepath.Join(userDataDir, "Profile *", "Preferences"))
	if err == nil && len(matches) == 1 {
		return filepath.Base(filepath.Dir(matches[0]))
	}
	return ""
}

func readDevToolsPort(dir string) (int, error) {
	b, err := os.ReadFile(filepath.Join(dir, "DevToolsActivePort"))
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 {
		return 0, errors.New("empty DevToolsActivePort")
	}
	port, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || port <= 0 {
		return 0, errors.New("invalid DevToolsActivePort")
	}
	return port, nil
}

func waitDevToolsPort(ctx context.Context, dir string, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	file := filepath.Join(dir, "DevToolsActivePort")
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		b, err := os.ReadFile(file)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			if len(lines) > 0 {
				port, perr := strconv.Atoi(strings.TrimSpace(lines[0]))
				if perr == nil && port > 0 {
					return port, nil
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("Chrome did not expose DevToolsActivePort within %s", timeout)
}

func openPage(ctx context.Context, port int, targetURL string) (*websocket.Conn, error) {
	u := fmt.Sprintf("http://127.0.0.1:%d/json/new?%s", port, url.QueryEscape(targetURL))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Chrome /json/new returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var target struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(b, &target); err != nil {
		return nil, err
	}
	if target.WebSocketDebuggerURL == "" {
		return nil, errors.New("Chrome did not return a page websocket URL")
	}
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, target.WebSocketDebuggerURL, nil)
	return ws, err
}

func (b *Browser) Evaluate(ctx context.Context, expression string) (json.RawMessage, error) {
	if b == nil || b.page == nil {
		return nil, errors.New("browser is not connected")
	}
	params := map[string]any{
		"expression":    expression,
		"awaitPromise":  true,
		"returnByValue": true,
	}
	raw, err := b.page.send(ctx, "Runtime.evaluate", params)
	if err != nil {
		return nil, err
	}
	var res struct {
		Result struct {
			Type        string          `json:"type"`
			Value       json.RawMessage `json:"value"`
			Description string          `json:"description"`
		} `json:"result"`
		ExceptionDetails any `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	if res.ExceptionDetails != nil {
		return nil, fmt.Errorf("browser evaluation failed: %s", string(raw))
	}
	if len(res.Result.Value) == 0 {
		return []byte("null"), nil
	}
	return res.Result.Value, nil
}

func (b *Browser) Navigate(ctx context.Context, targetURL string) error {
	if b == nil || b.page == nil {
		return errors.New("browser is not connected")
	}
	if _, err := b.page.send(ctx, "Page.navigate", map[string]any{"url": targetURL}); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := b.Evaluate(ctx, `document.readyState`)
		if err == nil && (string(raw) == `"complete"` || string(raw) == `"interactive"`) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

func (b *Browser) Cookies(ctx context.Context, urls ...string) ([]Cookie, error) {
	raw, err := b.page.send(ctx, "Network.getCookies", map[string]any{"urls": urls})
	if err != nil {
		return nil, err
	}
	var res struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return res.Cookies, nil
}

func (b *Browser) SetCookies(ctx context.Context, cookies []Cookie) error {
	raw := make([]map[string]any, 0, len(cookies))
	for _, c := range cookies {
		if c.Name == "" || c.Value == "" {
			continue
		}
		item := map[string]any{
			"name":  c.Name,
			"value": c.Value,
			"url":   "https://www.linkedin.com",
			"path":  "/",
		}
		if c.Domain != "" {
			item["domain"] = c.Domain
		}
		if c.Path != "" {
			item["path"] = c.Path
		}
		if c.URL != "" {
			item["url"] = c.URL
		}
		raw = append(raw, item)
	}
	if len(raw) == 0 {
		return nil
	}
	_, err := b.page.send(ctx, "Network.setCookies", map[string]any{"cookies": raw})
	return err
}

func (b *Browser) Close() error {
	if b == nil || b.page == nil || b.page.ws == nil {
		return nil
	}
	return b.page.ws.Close()
}

type cdpConn struct {
	ws   *websocket.Conn
	mu   sync.Mutex
	next int
}

func (c *cdpConn) send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	id := c.next
	msg := map[string]any{"id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.ws.SetWriteDeadline(deadline)
		_ = c.ws.SetReadDeadline(deadline)
	} else {
		_ = c.ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
		_ = c.ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	if err := c.ws.WriteJSON(msg); err != nil {
		return nil, err
	}
	for {
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := c.ws.ReadJSON(&resp); err != nil {
			return nil, err
		}
		if resp.ID != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP %s failed: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}
