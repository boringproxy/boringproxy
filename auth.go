package main

import (
	"log"
        "fmt"
        "errors"
        "crypto/rand"
        "math/big"
        "net/smtp"
        "sync"
        "time"
        "io/ioutil"
        "encoding/json"
)


type Auth struct {
        pendingRequests map[string]chan struct{}
        sessions map[string]*Session
        mutex *sync.Mutex
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

        pendingRequests := make(map[string]chan struct{})
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

func (a *Auth) Login(email string, config *BoringProxyConfig, canceled <-chan struct{}) (string, error) {

        key, err := genRandomKey()
        if err != nil {
                return "", errors.New("Error generating key")
        }

        link := fmt.Sprintf("https://%s/verify?key=%s", config.AdminDomain, key)

        bodyTemplate := "From: %s <%s>\r\n" +
                "To: %s\r\n" +
                "Subject: Email Verification\r\n" +
                "\r\n" +
                "This is an email verification request from %s. Please click the following link to complete the verification:\r\n" +
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

        doneChan := make(chan struct{})

        a.mutex.Lock()
        a.pendingRequests[key] = doneChan
        a.mutex.Unlock()

        // Starting timeout here after sending the email, because sometimes it takes
        // a few seconds to send.
        timeout := time.After(time.Duration(60) * time.Second)

        select {
        case <-doneChan:
                token, err := genRandomKey()
                if err != nil {
                        return "", errors.New("Error generating key")
                }

                a.mutex.Lock()
                a.sessions[token] = &Session{Id: email}
                a.mutex.Unlock()

                saveJson(a.sessions, "sessions.json")

                return token, nil
        case <-timeout:
                a.mutex.Lock()
                delete(a.pendingRequests, key)
                a.mutex.Unlock()
                return "", errors.New("Timeout")
        case <-canceled:
                a.mutex.Lock()
                delete(a.pendingRequests, key)
                a.mutex.Unlock()
        }

        return "", nil
}

func (a *Auth) Verify(key string) error {
        a.mutex.Lock()
        defer a.mutex.Unlock()

        doneChan, ok := a.pendingRequests[key]

        if !ok {
                return errors.New("No pending request for that key. It may have expired.")
        }

        delete(a.pendingRequests, key)
        close(doneChan)

        return nil
}


const chars string = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";
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
