package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"github.com/harry2855/main.go/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"time"
)

type apiConfig struct {
	fileserverHits int
	queries *database.Queries
	platform string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string	`json:"body"`
	UserID    uuid.UUID	`json:"user_id"`
}

func handleHealthz(w http.ResponseWriter,r *http.Request){
	w.Header().Set("Content-Type","text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) countHandler(w http.ResponseWriter,r *http.Request){
	w.Header().Add("Content-Type", "text/html")
	hitsStr := strconv.Itoa(cfg.fileserverHits)
	htmlContent := `<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited ` + hitsStr + ` times!</p>
		</body>
	</html>`

	w.Write([]byte(htmlContent))
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	// ...
	
	return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		cfg.fileserverHits++
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w,r)
		
	})
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter,r* http.Request){
	type parameters struct{
		Email string `json:"email"`
	}
	Param := parameters{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	user, _ := cfg.queries.CreateUser(r.Context(), Param.Email) // r.Context() -> handles timeouts
	// Here Param.Email corresponds to the $1, $1 means the firsrt of the argument leaving context

	resBody := user
	dat, err := json.Marshal(resBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		
		return
	}
	w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(201)
	w.Write(dat)

}

func (cfg *apiConfig) deleteUsersHandler(w http.ResponseWriter,r *http.Request){
	if cfg.platform!="dev"{
		w.WriteHeader(403)
		return 
	}
	err := cfg.queries.DeleteAllUsers(r.Context())
	if err!=nil {
		log.Printf("Error deleting users: %s",err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(200)
}

func (cfg *apiConfig) chirpHandler(w http.ResponseWriter, r*http.Request){
	type parameters struct{
		Body string `json:"body"`
		UserId string `json:"user_id"`
	}
	Param := parameters{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	if len(Param.Body)>140 {
		type returnVal struct{
			Error string `json:"error"`
		}
		respBody := returnVal{"Chirp is too long"}
		dat, err := json.Marshal(respBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
    	w.WriteHeader(400)
		w.Write(dat)
	} else {
		var s string = Param.UserId
		userID, err := uuid.Parse(s)
		if err != nil {
			log.Printf("Error parsing user ID: %s", err)
			w.WriteHeader(400)
			return
		}
		chirp, _ := cfg.queries.CreateChirp(r.Context(), database.CreateChirpParams{Body: Param.Body, UserID: userID})
		resBody := chirp
		dat, err := json.Marshal(resBody)
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)		
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(dat)
		
	}

}

func (cfg *apiConfig) GetChirpsHandler(w http.ResponseWriter,r *http.Request){
	chirps,_ := cfg.queries.GetChirps(r.Context())
	resBody := chirps
	dat, err := json.Marshal(resBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)		
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *apiConfig) GetChirpHandler(w http.ResponseWriter,r *http.Request){
	chirpString := r.PathValue("chirpID")
	chirpID,_ := uuid.Parse(chirpString)
	chirp,_ := cfg.queries.GetChirp(r.Context(),chirpID)
	if chirp.ID == uuid.Nil {
		w.WriteHeader(403)
		w.Header().Set("Content-Type","text/plain; charset=utf-8")
		w.Write([]byte("Chirp not found"))
		return 
	}
	fmt.Println(chirp)
	resBody := chirp
	dat, err := json.Marshal(resBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)		
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)
}

func main(){
	godotenv.Load() // without this line os.Getenv won't work
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	fmt.Println("fsf",dbURL)
	db, _ := sql.Open("postgres", dbURL)   // makes a connection with postgre server
	defer db.Close()
	dbQueries := database.New(db)
	fmt.Println(dbQueries)
	cfg := apiConfig{0,dbQueries,platform}
	mux := http.NewServeMux()
	mux.Handle("/app/",cfg.middlewareMetricsInc(http.StripPrefix("/app/",http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz",handleHealthz)
	mux.HandleFunc("GET /admin/metrics",cfg.countHandler)
	mux.HandleFunc("/api/reset",cfg.resetHandler)
	mux.HandleFunc("POST /api/users",cfg.createUserHandler)
	mux.HandleFunc("POST /admin/reset",cfg.deleteUsersHandler)
	mux.HandleFunc("POST /api/chirps",cfg.chirpHandler)
	mux.HandleFunc("GET /api/chirps",cfg.GetChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{chirpID}",cfg.GetChirpHandler)

	server := &http.Server{Addr: ":8080",Handler: mux}	
	server.ListenAndServe()
}