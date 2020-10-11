package main

import (
	"fmt"
	"github.com/GeertJohan/go.rice"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
)


type IndexData struct {
        Styles template.CSS
        Tunnels map[string]Tunnel
}

type ConfirmData struct {
        Styles template.CSS
        Message string
        ConfirmUrl string
        CancelUrl string
}

func (p *BoringProxy) handleAdminRequest(w http.ResponseWriter, r *http.Request) {

        box, err := rice.FindBox("webui")
        if err != nil {
                w.WriteHeader(500)
                io.WriteString(w, "Error opening webui")
                return
        }

        stylesText, err := box.String("styles.css")
        if err != nil {
                w.WriteHeader(500)
                io.WriteString(w, "Error reading styles.css")
                return
        }

	switch r.URL.Path {
	case "/login":
		p.handleLogin(w, r)
	case "/":

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

                indexData := IndexData {
                        Styles: template.CSS(stylesText),
                        Tunnels: p.db.GetTunnels(),
                }

		tmpl.Execute(w, indexData)

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

        case "/confirm-delete-tunnel":
		box, err := rice.FindBox("webui")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error opening webui")
			return
		}

                confirmTemplate, err := box.String("confirm.tmpl")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error reading confirm.tmpl")
			return
		}

		tmpl, err := template.New("test").Parse(confirmTemplate)
		if err != nil {
			w.WriteHeader(500)
			log.Println(err)
			io.WriteString(w, "Error compiling confirm.tmpl")
			return
		}

                r.ParseForm()

		if len(r.Form["domain"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid domain parameter"))
			return
		}
		domain := r.Form["domain"][0]

                data := &ConfirmData{
                        Styles: template.CSS(stylesText),
                        Message: fmt.Sprintf("Are you sure you want to delete %s?", domain),
                        ConfirmUrl: fmt.Sprintf("/delete-tunnel?domain=%s", domain),
                        CancelUrl: "/",
                }

		tmpl.Execute(w, data)


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

		if len(r.Form["domain"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid domain parameter"))
			return
		}
		domain := r.Form["domain"][0]

		p.tunMan.DeleteTunnel(domain)

		http.Redirect(w, r, "/", 307)
	default:
		w.WriteHeader(400)
		w.Write([]byte("Invalid endpoint"))
		return
	}
}

func (p *BoringProxy) handleTunnels(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "POST":
		p.handleCreateTunnel(w, r)
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /tunnels"))
		return
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

	if len(r.Form["domain"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid domain parameter"))
		return
	}
	domain := r.Form["domain"][0]

	if len(r.Form["client-name"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid client-name parameter"))
		return
	}
	clientName := r.Form["client-name"][0]

	if len(r.Form["client-port"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid client-port parameter"))
		return
	}

	clientPort, err := strconv.Atoi(r.Form["client-port"][0])
	if err != nil {
		w.WriteHeader(400)
		w.Write([]byte("Invalid client-port parameter"))
		return
	}

	fmt.Println(domain, clientName, clientPort)
	_, err = p.tunMan.CreateTunnelForClient(domain, clientName, clientPort)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, err.Error())
		return
	}

	http.Redirect(w, r, "/", 303)
}
