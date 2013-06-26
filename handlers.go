package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Stolen from net/http
func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() {
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && modtime.Before(t.Add(1*time.Second)) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}

func canGzip(req *http.Request) bool {
	acceptable := req.Header.Get("accept-encoding")
	return strings.Contains(acceptable, "gzip")
}

func serveSSI(w http.ResponseWriter, req *http.Request, root, path string, fi os.FileInfo) {
	data, err := processSSI(root, path)
	if err != nil {
		http.Error(w, "500 Internal Server Error",
			http.StatusInternalServerError)
		return
	}

	if checkLastModified(w, req, fi.ModTime()) {
		return
	}

	z := canGzip(req)

	w.Header().Set("Content-type", "text/html")
	if z {
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.WriteHeader(200)

	if z {
		buf := bytes.NewReader(data)
		gz := gzip.NewWriter(w)
		defer gz.Close()

		_, err := io.Copy(gz, buf)
		if err != nil {
			log.Printf("Error writing gzip things: %v", err)
		}
	} else {
		w.Write(data)
	}
}

func exists(path string) (bool, os.FileInfo) {
	fi, err := os.Stat(path)
	return err == nil, fi
}

type gzWriter struct {
	w http.ResponseWriter
	g io.WriteCloser
}

func (g *gzWriter) Header() http.Header {
	return g.w.Header()
}

func (g *gzWriter) Write(b []byte) (int, error) {
	return g.g.Write(b)
}

func (g *gzWriter) WriteHeader(c int) {
	g.w.WriteHeader(c)
}

func serveFile(w http.ResponseWriter, req *http.Request, path string) {
	ctype := mime.TypeByExtension(filepath.Ext(path))
	if strings.HasPrefix(ctype, "text/") && canGzip(req) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := &gzWriter{w, gzip.NewWriter(w)}
		defer gz.g.Close()
		http.ServeFile(gz, req, path)
	} else {
		http.ServeFile(w, req, path)
	}
}

func dirHandler(prefix, root string, showIndex bool) routeHandler {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		upath := req.URL.Path[len(prefix)-1:]
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			req.URL.Path = upath
		}

		finalpath := filepath.Join(root, upath)

		fi, err := os.Stat(finalpath)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if strings.HasSuffix(finalpath, ".shtml") {
			serveSSI(w, req, root, finalpath, fi)
			return
		}

		if fi.IsDir() {
			if !strings.HasSuffix(upath, "/") {
				http.Redirect(w, req, upath+"/",
					http.StatusMovedPermanently)
				return
			}

			ssiindex := filepath.Join(finalpath, "index.shtml")
			if ok, st := exists(ssiindex); ok {
				serveSSI(w, req, root, ssiindex, st)
				return
			}

			regularIndex := filepath.Join(finalpath, "index.html")
			if ok, _ := exists(regularIndex); ok {
				serveFile(w, req, finalpath)
				return
			} else {
				if !showIndex {
					http.Error(w, "403 Forbidden",
						http.StatusForbidden)
					return
				}
			}
		}

		serveFile(w, req, finalpath)
	}
}

func fileHandler(prefix, root string) routeHandler {
	return dirHandler(prefix, root, false)
}

func restrictedProxyHandler(prefix, dest string,
	methods []string) routeHandler {

	target, err := url.Parse(dest)
	if err != nil {
		log.Panicf("Error setting up handler with dest=%v:  %v",
			dest, err)
	}

	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		if len(prefix) >= 0 && len(prefix) < len(req.URL.Path) {
			req.URL.Path = req.URL.Path[len(prefix):]
		}
		req.URL.Path = target.Path + req.URL.Path
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
	}
	proxy := httputil.ReverseProxy{Director: director}

	return func(w http.ResponseWriter, req *http.Request) {
		req.Header.Set("Host", req.URL.Host)
		if len(methods) > 0 {
			for _, m := range methods {
				if m == req.Method {
					proxy.ServeHTTP(w, req)
					return
				}
			}
			http.Error(w, "405 Method Not Allowed",
				http.StatusMethodNotAllowed)
		} else {
			proxy.ServeHTTP(w, req)
		}
	}
}

func proxyHandler(prefix, dest string) routeHandler {
	return restrictedProxyHandler(prefix, dest, []string{})
}
