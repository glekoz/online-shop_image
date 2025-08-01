package fileserver

import (
	"fmt"
	"net/http"
	"time"
)

type FileServer struct {
	port int
	path string //some/path/static
}

func (s *FileServer) Run() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%v", s.port),
		Handler:      s.Routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	err := srv.ListenAndServe()
	return err
}

func (s *FileServer) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(s.path))
	mux.Handle("GET /static/", http.StripPrefix("/static", fs))
	return mux
}
