package main

import (
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
