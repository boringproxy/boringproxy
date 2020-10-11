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

type WebUiHandler struct {
	config *BoringProxyConfig
	db     *Database
	auth   *Auth
	tunMan *TunnelManager
}

type IndexData struct {
	Styles  template.CSS
	Tunnels map[string]Tunnel
}

type ConfirmData struct {
	Styles     template.CSS
	Message    string
	ConfirmUrl string
	CancelUrl  string
}

type LoginData struct {
	Styles template.CSS
}

func NewWebUiHandler(config *BoringProxyConfig, db *Database, auth *Auth, tunMan *TunnelManager) *WebUiHandler {
	return &WebUiHandler{
		config,
		db,
		auth,
		tunMan,
	}
}

func (h *WebUiHandler) handleWebUiRequest(w http.ResponseWriter, r *http.Request) {

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
		h.handleLogin(w, r)
	case "/":

		token, err := extractToken("access_token", r)
		if err != nil {

			loginTemplateStr, err := box.String("login.tmpl")
			if err != nil {
				log.Println(err)
				w.WriteHeader(500)
				io.WriteString(w, "Error reading login.tmpl")
				return
			}

			loginTemplate, err := template.New("test").Parse(loginTemplateStr)
			if err != nil {
				w.WriteHeader(500)
				log.Println(err)
				io.WriteString(w, "Error compiling login.tmpl")
				return
			}

			loginData := LoginData{
				Styles: template.CSS(stylesText),
			}

			w.WriteHeader(401)
			loginTemplate.Execute(w, loginData)
			return
		}

		if !h.auth.Authorized(token) {
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

		indexData := IndexData{
			Styles:  template.CSS(stylesText),
			Tunnels: h.db.GetTunnels(),
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

		if !h.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		h.handleTunnels(w, r)

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
			Styles:     template.CSS(stylesText),
			Message:    fmt.Sprintf("Are you sure you want to delete %s?", domain),
			ConfirmUrl: fmt.Sprintf("/delete-tunnel?domain=%s", domain),
			CancelUrl:  "/",
		}

		tmpl.Execute(w, data)

	case "/delete-tunnel":
		token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !h.auth.Authorized(token) {
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

		h.tunMan.DeleteTunnel(domain)

		http.Redirect(w, r, "/", 307)
	default:
		w.WriteHeader(400)
		w.Write([]byte("Invalid endpoint"))
		return
	}
}

func (h *WebUiHandler) handleTunnels(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case "POST":
		h.handleCreateTunnel(w, r)
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /tunnels"))
		return
	}
}

func (h *WebUiHandler) handleLogin(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for login"))
	}

	r.ParseForm()

	tokenList, ok := r.Form["token"]

	if !ok {
		w.WriteHeader(400)
		w.Write([]byte("Token required for login"))
		return
	}

	token := tokenList[0]

	if h.auth.Authorized(token) {
		cookie := &http.Cookie{Name: "access_token", Value: token, Secure: true, HttpOnly: true}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/", 303)
	} else {
		w.WriteHeader(401)
		w.Write([]byte("Invalid token"))
		return
	}
}

func (h *WebUiHandler) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {

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
	_, err = h.tunMan.CreateTunnelForClient(domain, clientName, clientPort)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, err.Error())
		return
	}

	http.Redirect(w, r, "/", 303)
}
