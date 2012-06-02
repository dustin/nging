package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
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
	ts  time.Time
	lw  *logWriter
	req *http.Request
}

func commonLog(outpath string, ch chan loggable) {

	logfile, err := os.OpenFile(outpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Could not open log file: %v", err)
	}
	defer logfile.Close()

	for l := range ch {
		ts := l.ts.Format("[02/Jan/2006:15:04:05 -0700]")
		h, _, err := net.SplitHostPort(l.req.RemoteAddr)
		if err != nil {
			log.Printf("Couldn't split %v", l.req.RemoteAddr)
			h = l.req.RemoteAddr
		}

		fmt.Fprintf(logfile,
			`%s - - %s "%s %s %s" %d %d "-" "%s" %s`+"\n",
			h, ts, l.req.Method, l.req.URL.Path,
			l.req.Proto, l.lw.status, l.lw.written,
			l.req.Header.Get("User-Agent"), l.req.Host)
	}
}
