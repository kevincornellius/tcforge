package handler

import (
	"log"
	"os"
)

var jwtSecret []byte

func InitJWT() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "set_this_pls"
		log.Println("warning: JWT_SECRET not set — using default, please set it in production")
	}
	jwtSecret = []byte(secret)
}
