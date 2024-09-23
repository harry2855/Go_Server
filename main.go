package main

import (
	"net/http"
	"strconv"
)

type apiConfig struct {
	fileserverHits int
}


func handleHealthz(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type","text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) countHandler(w http.ResponseWriter,r *http.Request){
	w.Header().Add("Content-Type","text/plain; charset=utf-8")
	hitsStr := strconv.Itoa(cfg.fileserverHits)
	w.Write([]byte("Hits: "+hitsStr))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	// ...
	
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		cfg.fileserverHits++
		next.ServeHTTP(w,r)
	})
}



func main(){
	cfg := apiConfig{0}
	mux := http.NewServeMux()
	mux.Handle("/app/",cfg.middlewareMetricsInc(http.StripPrefix("/app/",http.FileServer(http.Dir(".")))))
	mux.HandleFunc("/healthz",handleHealthz)
	mux.HandleFunc("/metrics",cfg.countHandler)
	mux.HandleFunc("/reset",cfg.resetHandler)

	server := &http.Server{Addr: ":8080",Handler: mux}	
	server.ListenAndServe()
}