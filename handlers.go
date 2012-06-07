package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func serveSSI(w http.ResponseWriter, req *http.Request, root, path string) {
	data, err := processSSI(root, path)
	if err != nil {
		http.Error(w, "500 Internal Server Error",
			http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-type", "text/html")
	w.WriteHeader(200)
	w.Write(data)

}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirHandler(prefix, root string, showIndex bool) routeHandler {
	return func(w http.ResponseWriter, req *http.Request) {
		upath := req.URL.Path[len(prefix)-1:]
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			req.URL.Path = upath
		}

		finalpath := filepath.Join(root, upath)

		if strings.HasSuffix(finalpath, ".shtml") {
			serveSSI(w, req, root, finalpath)
			return
		}

		fi, err := os.Stat(finalpath)
		if err != nil {
			http.NotFound(w, req)
			return
		}

		if fi.IsDir() {
			if !strings.HasSuffix(upath, "/") {
				http.Redirect(w, req, upath+"/",
					http.StatusMovedPermanently)
				return
			}

			ssiindex := filepath.Join(finalpath, "index.shtml")
			if exists(ssiindex) {
				serveSSI(w, req, root, ssiindex)
				return
			}

			regularIndex := filepath.Join(finalpath, "index.html")
			if exists(regularIndex) {
				http.ServeFile(w, req, finalpath)
				return
			} else {
				if !showIndex {
					http.Error(w, "403 Forbidden",
						http.StatusForbidden)
					return
				}
			}
		}

		http.ServeFile(w, req, finalpath)
	}
}

func fileHandler(prefix, root string) routeHandler {
	return dirHandler(prefix, root, false)
}

func proxyHandler(prefix, dest string) routeHandler {
	target, err := url.Parse(dest)
	if err != nil {
		log.Panicf("Error setting up handler with dest=%v:  %v",
			dest, err)
	}

	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = req.URL.Path[len(prefix)-1:]
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
	}
	proxy := httputil.ReverseProxy{Director: director}

	return func(w http.ResponseWriter, req *http.Request) {
		req.Header.Set("Host", req.URL.Host)
		proxy.ServeHTTP(w, req)
	}
}
