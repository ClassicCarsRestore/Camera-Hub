package main

import (
	"github.com/gin-gonic/gin"
	"time"
	"net/http"
    "bytes"
    "encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
)

var jwtKey = []byte("secret_key")

const secret = "yoursecret"

type Claims struct {
	Id string `json:"id"`
	StartTime string `json:"startTime"`
	EndTime string `json:"endTime"`
	jwt.StandardClaims
}

type Login struct {
	ProjectName string `json:"projectName"`
	Password string `json:"password"`
}

type Response struct {
	Id string `json:"id"`
	StartTime string `json:"startTime"`
	EndTime string `json:"endTime"`
}

func Authenticate(c *gin.Context) bool {
	var logCredentials Login
    if err := c.ShouldBindJSON(&logCredentials); err != nil {
      return false
    }
	json_data, err := json.Marshal(logCredentials)

	req, err := http.NewRequest(http.MethodPost, monitorURL + "/api/CameraHub/Authenticate", bytes.NewReader(json_data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", secret)

	resp, err := http.DefaultClient.Do(req)

	if err != nil || resp.StatusCode >= 400 {
      return false
    }

    responseData, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        fmt.Print(err.Error())
        return false
    }

	var r Response
	json.Unmarshal([]byte(responseData), &r)

	startTime,_:= time.Parse("2006-01-02T15:04:05.000Z", r.StartTime)
	endTime,_:= time.Parse("2006-01-02T15:04:05.000Z", r.EndTime)

	expirationTime := endTime
	claims := &Claims{
		Id: r.Id,
		StartTime: startTime.String(),
		EndTime: endTime.String(),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)

	if err == nil {
		c.SetCookie("Authorization", tokenString, int(endTime.Sub(time.Now()).Seconds()), "/", "", false, false)
	}
	if time.Now().After(startTime) && time.Now().Before(endTime){
		return true
	}
	return false
}

func Authorize(cookie string) bool {

	token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})

	if err != nil{
		return false
	}

	now := time.Now().UTC()
	if claims, ok := token.Claims.(*Claims); ok && token.Valid && claims.StartTime < now.String() && claims.EndTime > now.String() {
		return true
	}
	return false
}

func GetClaims(cookie string) *Claims {
	var claims *Claims

	token, err := jwt.ParseWithClaims(cookie, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtKey), nil
	})

	if err != nil{
		return claims
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims
	}
	return claims
}