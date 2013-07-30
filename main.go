package main

import (
	_ "expvar"
	"flag"
	"log"
	"log/syslog"
	"net"
	"net/http"
	"regexp"
	"syscall"
	"time"
)

type routeHandler func(w http.ResponseWriter, req *http.Request)

type routingEntry struct {
	Host    string
	Path    *regexp.Regexp
	Handler routeHandler
}

var emptyRegex = regexp.MustCompile("")

var routingTable []routingEntry = []routingEntry{

	routingEntry{"install.west.spy.net", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/install")},

	routingEntry{"media.west.spy.net", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/media")},

	routingEntry{"www.musicrivals.org", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/www.musicrivals.org")},

	routingEntry{"www.rockstarprogrammer.org",
		regexp.MustCompile("^/favicon.ico$"),
		fileHandler("/", "/data/web/purple-virts/rsp-static/media/favicon.ico")},
	routingEntry{"www.rockstarprogrammer.org", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/rsp-static")},

	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/therm/"),
		proxyHandler("/", "http://menudo:7777/therm/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/house/"),
		proxyHandler("/", "http://menudo.west.spy.net:7777/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/gitmirror/"),
		proxyHandler("/gitmirror/", "http://menudo:8124/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/eve/"),
		proxyHandler("/eve/", "http://eve:4984/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/mrtg/"),
		proxyHandler("/mrtg/", "http://cbfs:8484/public/mrtg/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/public/"),
		proxyHandler("/public/", "http://cbfs:8484/public/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/icons/"),
		proxyHandler("/icons/", "http://192.168.1.95/icons/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/s3sign/"),
		proxyHandler("/s3sign/", "http://eve:8123/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/washer"),
		proxyHandler("/", "http://eve:8223/")},
	routingEntry{"bleu.west.spy.net",
		regexp.MustCompile("^/~dustin/repo/"),
		errorHandler(410, "Repos are no longer served here")},
	routingEntry{"bleu.west.spy.net",
		regexp.MustCompile("^/~dustin/m2repo/"),
		errorHandler(410, "Repos are no longer served here")},
	routingEntry{"bleu.west.spy.net",
		regexp.MustCompile("^/~dustin/spyjar/"),
		errorHandler(410, "spy.jar docs are no longer served here")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/nging\\.git/"),
		proxyHandler("/nging.git/", "http://cbfs:8484/nging/nging.git/")},

	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/debug/vars"),
		defaultServerHandler},

	routingEntry{"seriesly.west.spy.net", regexp.MustCompile("^/"),
		restrictedProxyHandler("/", "http://bigdell.cbfs.west.spy.net:3133/",
			[]string{"GET", "HEAD"})},

	routingEntry{"r.west.spy.net", emptyRegex,
		proxyHandler("/", "http://menudo:8787/")},

	routingEntry{"", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/bleu/")},
}

func defaultServerHandler(w http.ResponseWriter, req *http.Request) {
	http.DefaultServeMux.ServeHTTP(w, req)
}

func findHandler(host, path string) routingEntry {
	for _, r := range routingTable {
		if r.Host == host || r.Host == "" {
			matches := r.Path.FindAllStringSubmatch(path, 1)
			if len(matches) > 0 {
				return r
			}
		}
	}
	log.Printf("Using default handler for %v %v", host, path)
	return routingEntry{"DEFAULT", nil,
		fileHandler("/", "/Users/dustin/Sites")}
}

type myHandler struct {
	ch chan loggable
}

func (h *myHandler) ServeHTTP(ow http.ResponseWriter, req *http.Request) {
	writer := &logWriter{ow, 0, 0}

	defer func(orig, q string) {
		h.ch <- loggable{time.Now(), writer, req, orig, q}
	}(req.URL.Path, req.URL.RawQuery)

	route := findHandler(req.Host, req.URL.Path)
	// log.Printf("Handling %v:%v", req.Method, req.URL.Path)
	route.Handler(writer, req)
}

func dropPrivs(uid, gid int, descriptors uint64) {
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE,
		&syscall.Rlimit{descriptors, descriptors})
	if err != nil {
		log.Printf("Couldn't set file limits: %v", err)
	}
	err = syscall.Setgid(gid)
	if err != nil {
		log.Fatalf("Error setting gid to %v: %v", gid, err)
	}
	err = syscall.Setuid(uid)
	if err != nil {
		log.Fatalf("Error setting uid to %v: %v", uid, err)
	}
}

func main() {
	addr := flag.String("addr", ":4984", "Address to bind to")
	logfile := flag.String("log", "access.log", "Access log path.")
	descriptors := flag.Uint64("descriptors", 256, "Descriptors to allow")
	uid := flag.Int("uid", -1, "UID to become.")
	gid := flag.Int("gid", -1, "GID to become.")
	useSyslog := flag.Bool("syslog", false, "Log to syslog")
	flag.Parse()

	if *useSyslog {
		sl, err := syslog.New(syslog.LOG_INFO, "nging")
		if err != nil {
			log.Fatalf("Error initializing syslog")
		}
		log.SetOutput(sl)
		log.SetFlags(0)
	}

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen:  %v", err)
	}

	if *uid > -1 && *gid > -1 {
		dropPrivs(*uid, *gid, *descriptors)
	}

	ch := make(chan loggable, 10000)
	if *useSyslog {
		go syslogLog(ch)
	} else {
		go commonLog(*logfile, ch)
	}

	s := &http.Server{
		Addr:        *addr,
		Handler:     &myHandler{ch},
		ReadTimeout: 30 * time.Second,
	}
	log.Printf("Listening to web requests on %s", *addr)
	log.Fatal(s.Serve(l))
}
