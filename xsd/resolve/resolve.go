// Package resolve supplies schema documents to xsd.Config.Resolve from a local directory, a cache directory and,
// only when it is turned on, the network.
//
// A document with the URL scheme://host/path is looked up as host/path: first in Options.Dir (read only, for
// example a folder of schemas shipped with a program or an embed.FS), then in Options.Cache (a directory on disk).
// With Options.Network on, a document found in neither is downloaded over HTTP or HTTPS and stored in the cache,
// so the next compilation reads it from disk:
//
//	set, err := xsd.Config{Resolve: resolve.New(resolve.Options{
//		Cache:      "schema-cache",
//		Network:    true,
//		AllowHosts: []string{"crd.gov.pl", "www.w3.org"},
//	})}.Compile(os.DirFS("schemas"), "profile.xsd")
//
// xsd.Set.Documents gives the URL and SHA-256 of every document the schema was compiled from, also of the
// downloaded ones. The package xsd never imports this package: a program that does not use it has no network code
// on the path of schema loading.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/polestorm/goxsdgo/xsd"
)

// Options configure a resolver. The zero value finds nothing: it is the same as no Config.Resolve.
type Options struct {
	// Dir, if not nil, is searched first, read only, with the host/path layout.
	Dir fs.FS
	// Cache, if not empty, is a directory searched after Dir, with the same layout. Downloaded documents are
	// written to it; it is created when needed.
	Cache string
	// Network turns on downloading documents found in neither Dir nor Cache. Off by default.
	Network bool
	// AllowHosts, if not empty, limits downloads (redirects included) to these hosts, compared without the port
	// and case-insensitively. Hosts outside the list are not found, as if Network were off.
	AllowHosts []string
	// Client is the HTTP client for downloads; nil means a client with Timeout and no cookies.
	Client *http.Client
	// Timeout bounds one download (default 30 s).
	Timeout time.Duration
	// MaxSize limits one downloaded document, in bytes (default and maximum xsd.MaxResolvedSize).
	MaxSize int64
}

// ErrDownload wraps every failed download (a network error, a status other than 200, a document too large). It
// stops the compilation: a missing document is reported as not found only when the network is off or the host
// is not allowed.
var ErrDownload = errors.New("resolve: download failed")

// New returns a function for xsd.Config.Resolve.
func New(o Options) func(xsd.SchemaRef) (io.ReadCloser, error) {
	if o.Timeout == 0 {
		o.Timeout = 30 * time.Second
	}
	if o.MaxSize <= 0 || o.MaxSize > xsd.MaxResolvedSize {
		o.MaxSize = xsd.MaxResolvedSize
	}
	return func(r xsd.SchemaRef) (io.ReadCloser, error) {
		u, err := url.Parse(r.Location)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fs.ErrNotExist
		}
		p, ok := LocalPath(u)
		if !ok {
			return nil, fs.ErrNotExist
		}
		if o.Dir != nil {
			if f, err := o.Dir.Open(p); err == nil {
				return f, nil
			}
		}
		if o.Cache != "" {
			if f, err := os.Open(filepath.Join(o.Cache, filepath.FromSlash(p))); err == nil {
				return f, nil
			}
		}
		if !o.Network || !o.allowed(u.Hostname()) {
			return nil, fs.ErrNotExist
		}
		data, err := o.download(u)
		if err != nil {
			return nil, err
		}
		if o.Cache != "" {
			if err := writeAtomic(filepath.Join(o.Cache, filepath.FromSlash(p)), data); err != nil {
				return nil, fmt.Errorf("resolve: caching %s: %w", r.Location, err)
			}
		}
		return io.NopCloser(strings.NewReader(string(data))), nil
	}
}

// LocalPath gives the path of a document with URL u in Dir and Cache: host/path, the host in lower case with
// ":" before a port replaced by "_". It reports false for a URL with a query, without a file name, or whose
// path leaves the host directory.
func LocalPath(u *url.URL) (string, bool) {
	if u.RawQuery != "" || u.Host == "" {
		return "", false
	}
	host := strings.ReplaceAll(strings.ToLower(u.Host), ":", "_")
	p := path.Clean("/" + u.Path)
	if p == "/" || strings.HasSuffix(u.Path, "/") {
		return "", false
	}
	full := host + p
	if !fs.ValidPath(full) || strings.ContainsAny(host, `/\`) {
		return "", false
	}
	return full, true
}

// allowed: host bez portu (url.URL.Hostname) jest na liście AllowHosts albo lista jest pusta.
func (o Options) allowed(host string) bool {
	if len(o.AllowHosts) == 0 {
		return true
	}
	for _, a := range o.AllowHosts {
		if strings.EqualFold(a, host) {
			return true
		}
	}
	return false
}

func (o Options) download(u *url.URL) ([]byte, error) {
	c := o.Client
	if c == nil {
		c = &http.Client{}
	}
	cc := *c
	prev := cc.CheckRedirect
	cc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || !o.allowed(req.URL.Hostname()) {
			return fmt.Errorf("redirect to %s not allowed", req.URL)
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if prev != nil {
			return prev(req, via)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDownload, u, err)
	}
	resp, err := cc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDownload, u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s: status %s", ErrDownload, u, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, o.MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrDownload, u, err)
	}
	if int64(len(data)) > o.MaxSize {
		return nil, fmt.Errorf("%w: %s: larger than %d bytes", ErrDownload, u, o.MaxSize)
	}
	return data, nil
}

// writeAtomic zapisuje plik przez plik tymczasowy w tym samym katalogu i zmianę nazwy: równoległe kompilacje nie
// zobaczą pliku zapisanego do połowy.
func writeAtomic(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".download-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(f.Name(), name)
	}
	if werr != nil {
		os.Remove(f.Name())
	}
	return werr
}
