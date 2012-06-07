package main

import (
	"flag"
	"log"
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
		proxyHandler("/therm/", "http://menudo.west.spy.net:7777/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/gitmirror/"),
		proxyHandler("/gitmirror/", "http://menudo:8124/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/eve/"),
		proxyHandler("/eve/", "http://eve/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/mrtg/"),
		proxyHandler("/", "http://eve/mrtg/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/icons/"),
		proxyHandler("/icons/", "http://192.168.1.95/icons/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/pivotal/"),
		proxyHandler("/pivotal", "http://localhost:8888/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/s3sign/"),
		proxyHandler("/s3sign/", "http://eve:8123/")},
	routingEntry{"bleu.west.spy.net", regexp.MustCompile("^/nging\\.git/"),
		fileHandler("/nging.git/", "/home/dustin/go/src/misc/nging/.git/")},

	routingEntry{"", emptyRegex,
		fileHandler("/", "/data/web/purple-virts/bleu/")},
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

	defer func() {
		h.ch <- loggable{time.Now(), writer, req}
	}()

	defer req.Body.Close()
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
	flag.Parse()

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal("Failed to listen:  %v", err)
	}

	if *uid > -1 && *gid > -1 {
		dropPrivs(*uid, *gid, *descriptors)
	}

	ch := make(chan loggable, 10000)
	go commonLog(*logfile, ch)

	s := &http.Server{
		Addr:    *addr,
		Handler: &myHandler{ch},
	}
	log.Printf("Listening to web requests on %s", *addr)
	log.Fatal(s.Serve(l))
}
