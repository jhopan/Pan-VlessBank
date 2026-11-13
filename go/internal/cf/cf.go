// Package cf is the cloudflared manager of the panel: binary resolution and
// download, tunnel login/list/create/delete, tunnel run/stop/status, generated
// config.tunnel.yml and the Cloudflare REST helpers. Port of cf.py.
package cf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CFCDN is the cloudflared release download prefix (cf.py _CF_CDN).
const CFCDN = "https://github.com/cloudflare/cloudflared/releases/latest/download/"

var (
	mu    sync.Mutex
	cfBin string

	dlProgress = -1
	dlError    = ""

	procLogin *child
	loginURL  string
	loginDone bool

	procTunnel    *child
	procWebTunnel *child
)

var (
	dirMu   sync.Mutex
	workDir = "."
)

// SetDir sets the working directory the panel uses for cloudflared.log,
// config.tunnel.yml and the bundled binary (cf.py HERE).
func SetDir(dir string) {
	dirMu.Lock()
	workDir = dir
	dirMu.Unlock()
}

// Dir returns the working directory set with SetDir.
func Dir() string {
	dirMu.Lock()
	defer dirMu.Unlock()
	return workDir
}

// BundlePath is where the bundled cloudflared lives (cf.py _bundle_path).
func BundlePath() string {
	return filepath.Join(Dir(), "cloudflared"+exeSuffix())
}

// FindBin mirrors cf.py find_bin: dir/cloudflared(.exe), else PATH.
func FindBin(dir string) string {
	b := filepath.Join(dir, "cloudflared"+exeSuffix())
	if fi, err := os.Stat(b); err == nil && !fi.IsDir() {
		return b
	}
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	return ""
}

// resolvedBin re-resolves the binary like cf.py _resolved_bin.
func resolvedBin() string {
	p := FindBin(Dir())
	mu.Lock()
	cfBin = p
	mu.Unlock()
	return p
}

// BinOK reports whether a cloudflared binary is available (cf.py bin_ok).
func BinOK() bool {
	return resolvedBin() != ""
}

// Version returns the first line of "cloudflared --version" (cf.py cf_version).
func Version() string {
	b := resolvedBin()
	if b == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, b, "--version")
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	text := strings.TrimSpace(out.String() + errb.String())
	if err != nil && text == "" {
		return "?"
	}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// PlatformInfo mirrors cf.py platform_info.
func PlatformInfo() map[string]any {
	raw := resolvedBin()
	bin := raw
	if bin == "" {
		bin = "(belum ditemukan)"
	}
	return map[string]any{
		"os":        osName(),
		"arch":      machineName(),
		"bin":       bin,
		"bin_found": raw != "",
		"bundle":    BundlePath(),
	}
}

// BundleName mirrors cf.py _cf_fname (the release asset name for this platform).
func BundleName() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "cloudflared-windows-amd64.exe", nil
	case "windows/386":
		return "cloudflared-windows-386.exe", nil
	case "linux/amd64":
		return "cloudflared-linux-amd64", nil
	case "linux/arm64":
		return "cloudflared-linux-arm64", nil
	case "linux/arm":
		return "cloudflared-linux-arm", nil
	case "darwin/amd64":
		return "cloudflared-darwin-amd64.tgz", nil
	case "darwin/arm64":
		return "cloudflared-darwin-arm64.tgz", nil
	}
	return "", fmt.Errorf("Platform tidak didukung: (%s, %s)", osName(), runtime.GOARCH)
}

// DownloadURL is the asset URL for this platform, "" when unsupported.
func DownloadURL() string {
	name, err := BundleName()
	if err != nil {
		return ""
	}
	return CFCDN + name
}

// DownloadBinary starts the background download (cf.py download_binary).
func DownloadBinary() {
	mu.Lock()
	if dlProgress >= 0 && dlProgress <= 100 {
		mu.Unlock()
		return
	}
	dlProgress = 0
	dlError = ""
	mu.Unlock()

	go func() {
		err := doDownload()
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			dlProgress = -2
			dlError = err.Error()
			return
		}
		dlProgress = 101
	}()
}

func doDownload() error {
	fname, err := BundleName()
	if err != nil {
		return err
	}
	dest := BundlePath()
	tmp := dest + ".tmp"
	if err := fetchTo(CFCDN+fname, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		os.Chmod(dest, 0o755)
	}
	resolvedBin()
	return nil
}

func fetchTo(url, path string) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, url)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	total := resp.ContentLength
	var count int64
	buf := make([]byte, 32*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			count += int64(n)
			if total > 0 {
				p := int(count * 100 / total)
				if p > 99 {
					p = 99
				}
				mu.Lock()
				dlProgress = p
				mu.Unlock()
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

// DownloadStatus mirrors cf.py dl_status.
func DownloadStatus() map[string]any {
	mu.Lock()
	p, e := dlProgress, dlError
	mu.Unlock()
	switch {
	case p == 101 || (p == -1 && BinOK()):
		return map[string]any{"state": "done", "progress": 100, "error": "", "version": Version()}
	case p == -2:
		return map[string]any{"state": "error", "progress": 0, "error": e, "version": ""}
	case p >= 0:
		return map[string]any{"state": "downloading", "progress": p, "error": "", "version": ""}
	}
	return map[string]any{"state": "missing", "progress": 0, "error": "", "version": ""}
}

// TailLog returns the last n lines of cloudflared.log (cf.py _tail_log).
func TailLog(n int) string {
	b, err := os.ReadFile(filepath.Join(Dir(), "cloudflared.log"))
	if err != nil {
		return ""
	}
	return tailLines(strings.ReplaceAll(string(b), "\r\n", "\n"), n)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	}
	return runtime.GOOS
}

// machineName mirrors Python's platform.machine() so the panel shows the same string.
func machineName() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "AMD64"
	case "windows/arm64":
		return "ARM64"
	case "windows/386":
		return "x86"
	case "linux/amd64":
		return "x86_64"
	case "linux/arm64":
		return "aarch64"
	case "linux/386":
		return "i686"
	case "darwin/amd64":
		return "x86_64"
	case "darwin/arm64":
		return "arm64"
	}
	return runtime.GOARCH
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func tailLines(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	lines := make([]string, 0, n)
	start := 0
	for i := range s {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "")
}

func jsonStr(v any) string {
	s, _ := v.(string)
	return s
}

func toInt(v any) (int, error) {
	switch t := v.(type) {
	case float64:
		return int(t), nil
	case int:
		return t, nil
	case string:
		t = strings.TrimSpace(t)
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0, err
		}
		return n, nil
	case bool:
		if t {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("bukan angka: %v", v)
}

// child is a started process whose exit is observed through a closed channel.
type child struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func startChild(cmd *exec.Cmd) (*child, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &child{cmd: cmd, done: make(chan struct{})}
	go func() {
		cmd.Wait()
		close(c.done)
	}()
	return c, nil
}

func (c *child) alive() bool {
	if c == nil {
		return false
	}
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

func (c *child) pid() int {
	return c.cmd.Process.Pid
}

func waitClosed(done <-chan struct{}, d time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}
