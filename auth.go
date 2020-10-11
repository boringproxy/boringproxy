package main

import (
	"crypto/rand"
	"math/big"
	"sync"
)

type Auth struct {
	db              *Database
	pendingRequests map[string]*LoginRequest
	mutex           *sync.Mutex
}

type LoginRequest struct {
	Email string
}

func NewAuth(db *Database) *Auth {

	pendingRequests := make(map[string]*LoginRequest)
	mutex := &sync.Mutex{}

	return &Auth{db, pendingRequests, mutex}
}

func (a *Auth) Authorized(token string) bool {
	_, exists := a.db.GetTokenData(token)

	if exists {
		return true
	}

	return false
}

const chars string = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func genRandomKey() (string, error) {
	id := ""
	for i := 0; i < 32; i++ {
		randIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		id += string(chars[randIndex.Int64()])
	}
	return id, nil
}
