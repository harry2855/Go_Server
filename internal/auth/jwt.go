package auth

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error){
	claims := jwt.RegisteredClaims{  // Payload is entered
		Issuer:    "chirpy",
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)  // Signing method is specified
	
	tokenString, err := token.SignedString([]byte(tokenSecret))  // Token is signed and the signed string is appended
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error){
	var userClaim jwt.RegisteredClaims;
	token, err := jwt.ParseWithClaims(tokenString,&userClaim,func(token *jwt.Token) (interface{}, error) {
		return []byte(tokenSecret), nil
	})
	log.Printf("%+v",token)
	fmt.Println(token.Claims.GetExpirationTime())
	if !token.Valid {  // token is invalid
		return uuid.Nil,errors.New("invalid token")
	}
	if err !=nil{   // any general error
		return uuid.Nil,err

	}
	stringId,_ := token.Claims.GetSubject()
	fmt.Println(stringId)

	Id,_ := uuid.Parse(stringId)
	fmt.Println(Id)
	return Id,nil
}

func GetBearerToken(headers http.Header) (string, error){  // extract JWT from headers
	fmt.Println(headers)
	tokenString := headers.Get("Authorization")
	fmt.Println(tokenString)
	s := "Bearer "
	if tokenString == "" {
		return "",errors.New("no JWT token provided")
	}
	return strings.TrimPrefix(tokenString,s),nil
}