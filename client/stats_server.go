package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"
)

type statEntry struct {
	Request   http.Request
	Response  http.Response
	Timestamp time.Time
	Took      time.Duration
}

type statServer struct {
	stats []statEntry
	t     *template.Template
}

func newStatServer(ctx context.Context, port string) *statServer {
	t, err := template.New("main").Parse(tpl)
	if err != nil {
		log.Println("failed to parse template", err)
	}
	ss := &statServer{
		stats: make([]statEntry, 0),
		t:     t,
	}

	server := &http.Server{Addr: port, Handler: ss}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println("failed to start the server", err)
		}
	}()
	go func() {
		<-ctx.Done()
		log.Println("context is closed, shutting down")
		server.Shutdown(context.TODO())
		return

	}()

	return ss
}

func (s *statServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("serving stats", len(s.stats))
	if len(s.stats) > 0 {
		log.Println(s.stats[0].Request.RemoteAddr)
	}
	data := map[string]interface{}{
		"stats": s.stats,
	}
	s.t.Execute(w, data)

}

func (s *statServer) collect(st statEntry) {
	s.stats = append(s.stats, st)
}
