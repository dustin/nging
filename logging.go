package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type logWriter struct {
	orig http.ResponseWriter

	written uint64
	status  int
}

func (lw *logWriter) Header() http.Header {
	return lw.orig.Header()
}

func (lw *logWriter) Write(b []byte) (int, error) {
	r, e := lw.orig.Write(b)
	lw.written += uint64(r)
	return r, e
}

func (lw *logWriter) WriteHeader(code int) {
	lw.status = code
	lw.orig.WriteHeader(code)
}

type loggable struct {
	ts    time.Time
	lw    *logWriter
	req   *http.Request
	ustr  string
	query string
}

func commonLog(outpath string, ch chan loggable) {

	logfile, err := os.OpenFile(outpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Could not open log file: %v", err)
	}
	defer logfile.Close()

	timer := time.Tick(time.Second)

	for l := range ch {
		ts := l.ts.Format("[02/Jan/2006:15:04:05 -0700]")
		h, _, err := net.SplitHostPort(l.req.RemoteAddr)
		if err != nil {
			log.Printf("Couldn't split %v", l.req.RemoteAddr)
			h = l.req.RemoteAddr
		}

		url, err := url.Parse(l.ustr)
		if err != nil {
			log.Printf("Couldn't parse url %v: %v", l.ustr, err)
			url = l.req.URL
		}

		pathPart := url.Path
		if l.query != "" {
			pathPart += "?" + l.query
		}

		fmt.Fprintf(logfile,
			`%s - - %s "%s %s %s" %d %d "-" "%s" %s`+"\n",
			h, ts, l.req.Method, pathPart,
			l.req.Proto, l.lw.status, l.lw.written,
			l.req.Header.Get("User-Agent"), l.req.Host)

		select {
		case <-timer:
			logfile.Sync()
		default:
		}
	}
}
