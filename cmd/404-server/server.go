/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// A webserver that only serves a 404 page. Used as a default backend.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// command line arguments
	port := flag.Int("port", 8080, "Port number to serve default backend 404 page.")
	timeout := flag.Duration("timeout", 5*time.Second, "Time in seconds to wait before forcefully terminating the server.")

	flag.Parse()

	notFound := newHTTPServer(fmt.Sprintf(":%d", *port), notFound())

	// start the main http server
	go func() {
		err := notFound.ListenAndServe()
		if err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "could not start http server: %s\n", err)
			os.Exit(1)
		}
	}()

	waitShutdown(notFound, *timeout)
}

type server struct {
	mux *http.ServeMux
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
	}
}

func notFound(options ...func(*server)) *server {
	s := &server{mux: http.NewServeMux()}
	// TODO: this handler exists only to avoid breaking existing deployments
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "default backend - 404")
	})
	return s
}

func waitShutdown(s *http.Server, timeout time.Duration) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Fprintf(os.Stdout, "stopping http server...\n")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "could not gracefully shutdown http server: %s\n", err)
	}
}
