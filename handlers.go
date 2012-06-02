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
		http.NotFound(w, req)
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

func fileHandler(root string) routeHandler {
	return func(w http.ResponseWriter, req *http.Request) {
		upath := req.URL.Path
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
		}

		http.ServeFile(w, req, finalpath)
	}
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
		log.Printf("Going to %v in target %v", req.URL, target)
	}
	proxy := httputil.ReverseProxy{Director: director}

	return func(w http.ResponseWriter, req *http.Request) {
		req.Header.Set("Host", req.URL.Host)
		proxy.ServeHTTP(w, req)
	}
}
