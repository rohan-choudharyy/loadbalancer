package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type Server interface {
	Address() string
	IsAlive() bool
	Serve(rw http.ResponseWriter, r *http.Request)
}

type server struct {
	addr  string
	proxy *httputil.ReverseProxy
}

func newserver(addr string) *server {
	serverUrl, err := url.Parse(addr)
	handleErr(err)

	return &server{
		addr: addr,
		proxy: &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = serverUrl.Scheme
				req.URL.Host = serverUrl.Host
				req.Host = serverUrl.Host
			},
		},
	}
}

type loadbalance struct {
	port    string
	rrcount int
	servers []Server
}

func newloadbalance(port string, servers []Server) *loadbalance {
	return &loadbalance{
		port:    port,
		rrcount: 0,
		servers: servers,
	}
}

func handleErr(err error) {
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func (s *server) Address() string { return s.addr }

func (s *server) IsAlive() bool { return true }

func (s *server) Serve(rw http.ResponseWriter, req *http.Request) {
	s.proxy.ServeHTTP(rw, req)
}

func (lb *loadbalance) getnextServer() Server {
	server := lb.servers[lb.rrcount%len(lb.servers)]
	for !server.IsAlive() {
		lb.rrcount++
		server = lb.servers[lb.rrcount%len(lb.servers)]
	}
	lb.rrcount++
	return server
}

func (lb *loadbalance) serveProxy(rw http.ResponseWriter, req *http.Request) {
	targetServer := lb.getnextServer()
	fmt.Printf("Forwarding request to address %q\n", targetServer.Address())
	targetServer.Serve(rw, req)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func main() {
	servers := []Server{
		newserver("https://www.google.com"),
		newserver("https://www.youtube.com"),
		newserver("https://www.wikipedia.org"),
	}
	lb := newloadbalance("8000", servers)
	handleRedirect := func(rw http.ResponseWriter, req *http.Request) {
		lb.serveProxy(rw, req)
	}
	http.HandleFunc("/", corsMiddleware(handleRedirect))

	fmt.Printf("Serving requests at 'localhost:%s'\n", lb.port)
	http.ListenAndServe(":"+lb.port, nil)
}
