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
	"github.com/harry2855/main.go/internal/auth"
)

type apiConfig struct {
	fileserverHits int
	queries *database.Queries
	platform string
	secret string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Token     string    `json:"token"` 		
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string	`json:"body"`
	UserID    uuid.UUID	`json:"user_id"`
}

type Login struct{
	Email string `json:"email"`
	Password string `json:"password"`
	Expiry *int `json:"expires_in_seconds,omitempty"`
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
	Param := Login{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	hashedPassword,_ := auth.HashPassword(Param.Password)

	user, _ := cfg.queries.CreateUser(r.Context(), database.CreateUserParams{Email: Param.Email,HashedPassword: hashedPassword}) // r.Context() -> handles timeouts
	// Here Param.Email corresponds to the $1, $1 means the firsrt of the argument leaving context
	
	resBody := mapdatabaseUsertoUser(user)
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
	}
	Param := parameters{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	fmt.Println(Param)
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
		token,_ := auth.GetBearerToken(r.Header)
		userID,err := auth.ValidateJWT(token,cfg.secret)
		if err != nil && err.Error() == "invalid token" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(401)
			w.Write([]byte("Invalid JWT"))
			return
		}
		fmt.Println(userID)
		chirp, _ := cfg.queries.CreateChirp(r.Context(), database.CreateChirpParams{Body: Param.Body, UserID: userID})
		resBody := chirp
		fmt.Println(resBody)
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

func (cfg *apiConfig) loginHandler(w http.ResponseWriter,r *http.Request){
	Param := Login{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	var expiry time.Duration
	if Param.Expiry == nil  || *Param.Expiry>60{
		expiry  = 1*time.Hour
	} else {
		expiry =  time.Duration(*Param.Expiry)* time.Minute 
	}
	user,err1 := cfg.queries.GetUserbyEmailId(r.Context(),Param.Email)
	err  = auth.CheckPasswordHash(Param.Password,user.HashedPassword)
	if err!=nil || err1!=nil {
		w.WriteHeader(401)
		w.Header().Set("Content-Type","text/plain; charset=utf-8")
		w.Write([]byte("Login Failed"))
		return 
	}
	resBody := mapdatabaseUsertoUser(user)
	token,_ := auth.MakeJWT(user.ID,cfg.secret,expiry)
	resBody.Token = token
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

func (cfg *apiConfig) updateHandler(w http.ResponseWriter,r *http.Request){
	Param := Login{}
	decoder:= json.NewDecoder(r.Body)
	err := decoder.Decode(&Param)
	if err!=nil{
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	token,err := auth.GetBearerToken(r.Header)
	if err!=nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(401)
		w.Write([]byte("No jwt token provided"))
		return
	}
	userID,err := auth.ValidateJWT(token,cfg.secret)
	if err != nil && err.Error() == "invalid token" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(401)
		w.Write([]byte("Invalid JWT"))
		return
	}
	hashed_password,_ := auth.HashPassword(Param.Password)
	user,_ := cfg.queries.UpdateUser(r.Context(),database.UpdateUserParams{Email: Param.Email,HashedPassword:hashed_password ,ID:userID})
	resBody := mapdatabaseUsertoUser(user)
	resBody.Token = token
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

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter,r *http.Request){
	chirpString := r.PathValue("chirpID")
	chirpID,_ := uuid.Parse(chirpString)
	token,err := auth.GetBearerToken(r.Header)
	if err!=nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(401)
		w.Write([]byte("No jwt token provided"))
		return
	}
	userID,err := auth.ValidateJWT(token,cfg.secret)
	if err != nil && err.Error() == "invalid token" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(401)
		w.Write([]byte("Invalid JWT"))
		return
	}
	chirp,_ :=cfg.queries.GetChirp(r.Context(),chirpID)
	if chirp.ID == uuid.Nil {
		w.WriteHeader(404)
		w.Header().Set("Content-Type","text/plain; charset=utf-8")
		w.Write([]byte("Chirp not found"))
		return 
	}
	if chirp.UserID != userID {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(403)
		w.Write([]byte("You are not the author of the chirp"))
		return 
	}
	_ = cfg.queries.DeleteChirp(r.Context(),chirpID)
}

func main(){
	godotenv.Load() // without this line os.Getenv won't work
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("SECRET")
	fmt.Println("fsf",dbURL)
	db, _ := sql.Open("postgres", dbURL)   // makes a connection with postgre server
	defer db.Close()
	dbQueries := database.New(db)
	fmt.Println(dbQueries)
	cfg := apiConfig{0,dbQueries,platform,secret}
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
	mux.HandleFunc("POST /api/login",cfg.loginHandler)
	mux.HandleFunc("PUT /api/users",cfg.updateHandler)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}",cfg.deleteChirpHandler)

	server := &http.Server{Addr: ":8080",Handler: mux}	
	server.ListenAndServe()
}

func mapdatabaseUsertoUser(datbaseuser database.User) User {
	return User{
		ID:        datbaseuser.ID,
        Email:     datbaseuser.Email,
		CreatedAt: datbaseuser.CreatedAt,
		UpdatedAt: datbaseuser.UpdatedAt,
		Token: "",
	}
}