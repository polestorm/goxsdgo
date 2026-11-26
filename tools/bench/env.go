package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// xmllint: ścieżka do xmllint (libxml2) używana przez perf i probe; flaga -xmllint, domyślnie z PATH.
var xmllint = "xmllint"

// environment opisuje, gdzie i czym zmierzono wyniki; zapisywane obok wyników każdego trybu
// (out/bench/environment-<tryb>.json), żeby pomiar dało się powtórzyć i porównać.
type environment struct {
	Mode        string            `json:"mode"`
	Time        string            `json:"time"`
	Commit      string            `json:"goxsdgo_commit"`
	Dirty       bool              `json:"goxsdgo_dirty"`
	GoVersion   string            `json:"go_version"`
	OS          string            `json:"os"`
	OSVersion   string            `json:"os_version"`
	Arch        string            `json:"arch"`
	CPU         string            `json:"cpu"`
	NumCPU      int               `json:"num_cpu"`
	LoadAverage string            `json:"load_average"`
	Xmllint     string            `json:"xmllint"`
	Modules     map[string]string `json:"modules"`
}

func run(name string, args ...string) string {
	b, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func collectEnv(root, mode string) environment {
	e := environment{Mode: mode, Time: time.Now().Format(time.RFC3339), GoVersion: runtime.Version(),
		OS: runtime.GOOS, Arch: runtime.GOARCH, NumCPU: runtime.NumCPU(), Modules: map[string]string{}}
	e.Commit = run("git", "-C", root, "rev-parse", "--short", "HEAD")
	e.Dirty = run("git", "-C", root, "status", "--porcelain", "--untracked-files=no") != ""
	switch runtime.GOOS {
	case "darwin":
		e.OSVersion = "macOS " + run("sw_vers", "-productVersion")
		e.CPU = run("sysctl", "-n", "machdep.cpu.brand_string")
		e.LoadAverage = strings.Trim(run("sysctl", "-n", "vm.loadavg"), "{} ")
	case "linux":
		e.OSVersion = "Linux " + run("uname", "-r")
		if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, l := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(l, "model name") {
					_, v, _ := strings.Cut(l, ":")
					e.CPU = strings.TrimSpace(v)
					break
				}
			}
		}
		if b, err := os.ReadFile("/proc/loadavg"); err == nil {
			f := strings.Fields(string(b))
			if len(f) >= 3 {
				e.LoadAverage = strings.Join(f[:3], " ")
			}
		}
	default:
		e.OSVersion = runtime.GOOS
	}
	v := run(xmllint, "--version")
	e.Xmllint, _, _ = strings.Cut(v, "\n")
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Replace != nil {
				e.Modules[d.Path] = d.Version + " => " + d.Replace.Path
			} else {
				e.Modules[d.Path] = d.Version
			}
		}
	}
	return e
}

func writeEnv(root, out, mode string) error {
	return writeJSON(out, "environment-"+mode+".json", collectEnv(root, mode))
}

// resolveXmllint zamienia nazwę z flagi na ścieżkę bezwzględną, jeśli jest w PATH.
func resolveXmllint(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if lp, err := exec.LookPath(p); err == nil {
		return lp
	}
	return p
}
