package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/smtp"
	"sync"
	//"time"
)

type Auth struct {
	pendingRequests map[string]*LoginRequest
	sessions        map[string]*Session
	mutex           *sync.Mutex
}

type LoginRequest struct {
	Email string
}

type Session struct {
	Id string `json:"id"`
}

func NewAuth() *Auth {

	sessionsJson, err := ioutil.ReadFile("sessions.json")
	if err != nil {
		log.Println("failed reading sessions.json")
		sessionsJson = []byte("{}")
	}

	var sessions map[string]*Session

	err = json.Unmarshal(sessionsJson, &sessions)
	if err != nil {
		log.Println(err)
		sessions = make(map[string]*Session)
	}

	pendingRequests := make(map[string]*LoginRequest)
	mutex := &sync.Mutex{}

	return &Auth{pendingRequests, sessions, mutex}
}

func (a *Auth) Authorized(token string) bool {
	a.mutex.Lock()
	_, exists := a.sessions[token]
	a.mutex.Unlock()

	if exists {
		return true
	}

	return false
}

func (a *Auth) Login(email string, config *BoringProxyConfig) (string, error) {

	key, err := genRandomKey()
	if err != nil {
		return "", errors.New("Error generating key")
	}

	link := fmt.Sprintf("https://%s/login?key=%s", config.AdminDomain, key)

	bodyTemplate := "From: %s <%s>\r\n" +
		"To: %s\r\n" +
		"Subject: Email Verification\r\n" +
		"\r\n" +
		"This is email verification request from %s. Please click the following link to complete the verification:\r\n" +
		"\r\n" +
		"%s\r\n"

	fromText := "boringproxy email verifier"
	fromEmail := fmt.Sprintf("auth@%s", config.AdminDomain)
	emailBody := fmt.Sprintf(bodyTemplate, fromText, fromEmail, email, config.AdminDomain, link)

	emailAuth := smtp.PlainAuth("", config.Smtp.Username, config.Smtp.Password, config.Smtp.Server)
	srv := fmt.Sprintf("%s:%d", config.Smtp.Server, config.Smtp.Port)
	msg := []byte(emailBody)
	err = smtp.SendMail(srv, emailAuth, fromEmail, []string{email}, msg)
	if err != nil {
		return "", errors.New("Sending email failed. Probably a bad email address.")
	}

	a.mutex.Lock()
	a.pendingRequests[key] = &LoginRequest{email}
	a.mutex.Unlock()

	return "", nil
}

func (a *Auth) Verify(key string) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	request, ok := a.pendingRequests[key]

	if !ok {
		return "", errors.New("No pending request for that key. It may have expired.")
	}

	delete(a.pendingRequests, key)

	token, err := genRandomKey()
	if err != nil {
		return "", errors.New("Error generating key")
	}

	a.sessions[token] = &Session{Id: request.Email}

	saveJson(a.sessions, "sessions.json")

	return token, nil
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
