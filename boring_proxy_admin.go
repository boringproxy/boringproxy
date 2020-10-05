package main

import (
	"io"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"html/template"
	"encoding/json"
	"github.com/GeertJohan/go.rice"
)


func (p *BoringProxy) handleAdminRequest(w http.ResponseWriter, r *http.Request) {

	switch r.URL.Path {
	case "/login":
		p.handleLogin(w, r)
	case "/":
		box, err := rice.FindBox("webui")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error opening webui")
			return
		}

		token, err := extractToken("access_token", r)
		if err != nil {

			loginTemplate, err := box.String("login.tmpl")
			if err != nil {
				log.Println(err)
				w.WriteHeader(500)
				io.WriteString(w, "Error reading login.tmpl")
				return
			}

			w.WriteHeader(401)
			io.WriteString(w, loginTemplate)
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		indexTemplate, err := box.String("index.tmpl")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error reading index.tmpl")
			return
		}

		tmpl, err := template.New("test").Parse(indexTemplate)
		if err != nil {
			w.WriteHeader(500)
			log.Println(err)
			io.WriteString(w, "Error compiling index.tmpl")
			return
		}

		tmpl.Execute(w, p.tunMan.tunnels)

		//io.WriteString(w, indexTemplate)

	case "/tunnels":

		token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		p.handleTunnels(w, r)

	case "/delete-tunnel":
		token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !p.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		r.ParseForm()

		if len(r.Form["host"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid host parameter"))
			return
		}
		host := r.Form["host"][0]

		p.tunMan.DeleteTunnel(host)

		http.Redirect(w, r, "/", 307)
	default:
		w.WriteHeader(400)
		w.Write([]byte("Invalid endpoint"))
		return
	}
}

func (p *BoringProxy) handleTunnels(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	if r.Method == "GET" {
		body, err := json.Marshal(p.tunMan.tunnels)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error encoding tunnels"))
			return
		}
		w.Write([]byte(body))
	} else if r.Method == "POST" {
		p.handleCreateTunnel(w, r)
	} else if r.Method == "DELETE" {
		if len(query["host"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid host parameter"))
			return
		}
		host := query["host"][0]

		p.tunMan.DeleteTunnel(host)
	}
}

func (p *BoringProxy) handleLogin(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "GET":
		query := r.URL.Query()
		key, exists := query["key"]

		if !exists {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Must provide key for verification")
			return
		}

		token, err := p.auth.Verify(key[0])

		if err != nil {
			w.WriteHeader(400)
			fmt.Fprintf(w, "Invalid key")
			return
		}

		cookie := &http.Cookie{Name: "access_token", Value: token, Secure: true, HttpOnly: true}
		http.SetCookie(w, cookie)

		http.Redirect(w, r, "/", 307)

	case "POST":

		r.ParseForm()

		toEmail, ok := r.Form["email"]

		if !ok {
			w.WriteHeader(400)
			w.Write([]byte("Email required for login"))
			return
		}

		// run in goroutine because it can take some time to send the
		// email
		go p.auth.Login(toEmail[0], p.config)

		io.WriteString(w, "Check your email to finish logging in")
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for login"))
	}
}

func (p *BoringProxy) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	if len(r.Form["host"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid host parameter"))
		return
	}
	host := r.Form["host"][0]

	if len(r.Form["port"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid port parameter"))
		return
	}

	port, err := strconv.Atoi(r.Form["port"][0])
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Invalid port parameter"))
		return
	}

	err = p.tunMan.SetTunnel(host, port)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, "Failed to get cert. Ensure your domain is valid")
		return
	}

	http.Redirect(w, r, "/", 303)
}
