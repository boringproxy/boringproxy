package main

import (
	"errors"
	"fmt"
	"github.com/GeertJohan/go.rice"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type WebUiHandler struct {
	config   *BoringProxyConfig
	db       *Database
	auth     *Auth
	tunMan   *TunnelManager
	box      *rice.Box
	headHtml template.HTML
}

type IndexData struct {
	Head    template.HTML
	Tunnels map[string]Tunnel
}

type ConfirmData struct {
	Head       template.HTML
	Message    string
	ConfirmUrl string
	CancelUrl  string
}

type AlertData struct {
	Head        template.HTML
	Message     string
	RedirectUrl string
}

type LoginData struct {
	Head template.HTML
}

type HeadData struct {
	Styles template.CSS
}

type UsersData struct {
	Head  template.HTML
	Users map[string]User
}

func NewWebUiHandler(config *BoringProxyConfig, db *Database, auth *Auth, tunMan *TunnelManager) *WebUiHandler {
	return &WebUiHandler{
		config: config,
		db:     db,
		auth:   auth,
		tunMan: tunMan,
	}
}

func (h *WebUiHandler) handleWebUiRequest(w http.ResponseWriter, r *http.Request) {

	box, err := rice.FindBox("webui")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Error opening webui")
		return
	}

	h.box = box

	stylesText, err := box.String("styles.css")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Error reading styles.css")
		return
	}

	headTmplStr, err := box.String("head.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Error reading head.tmpl")
		return
	}

	headTmpl, err := template.New("head").Parse(headTmplStr)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		io.WriteString(w, "Error compiling head.tmpl")
		return
	}

	var headBuilder strings.Builder
	headTmpl.Execute(&headBuilder, HeadData{Styles: template.CSS(stylesText)})

	h.headHtml = template.HTML(headBuilder.String())

	token, err := extractToken("access_token", r)
	if err != nil {
		h.sendLoginPage(w, r, 401)
		return
	}

	if !h.auth.Authorized(token) {
		h.sendLoginPage(w, r, 403)
		return
	}

	switch r.URL.Path {
	case "/login":
		h.handleLogin(w, r)
	case "/users":
		h.users(w, r)
	case "/confirm-delete-user":
		h.confirmDeleteUser(w, r)
	case "/delete-user":
		h.deleteUser(w, r)
	case "/":

		indexTemplate, err := box.String("index.tmpl")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error reading index.tmpl")
			return
		}

		tmpl, err := template.New("index").Parse(indexTemplate)
		if err != nil {
			w.WriteHeader(500)
			log.Println(err)
			io.WriteString(w, "Error compiling index.tmpl")
			return
		}

		indexData := IndexData{
			Head:    h.headHtml,
			Tunnels: h.db.GetTunnels(),
		}

		tmpl.Execute(w, indexData)

	case "/tunnels":

		h.handleTunnels(w, r)

	case "/confirm-delete-tunnel":

		r.ParseForm()

		if len(r.Form["domain"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid domain parameter"))
			return
		}
		domain := r.Form["domain"][0]

		tmpl, err := h.loadTemplate("confirm.tmpl")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, err.Error())
			return
		}

		data := &ConfirmData{
			Head:       h.headHtml,
			Message:    fmt.Sprintf("Are you sure you want to delete %s?", domain),
			ConfirmUrl: fmt.Sprintf("/delete-tunnel?domain=%s", domain),
			CancelUrl:  "/",
		}

		tmpl.Execute(w, data)

	case "/delete-tunnel":

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

	if r.Method != "GET" {
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for login"))
	}

	r.ParseForm()

	tokenList, ok := r.Form["access_token"]

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
		h.sendLoginPage(w, r, 403)
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

func (h *WebUiHandler) sendLoginPage(w http.ResponseWriter, r *http.Request, code int) {

	loginTemplateStr, err := h.box.String("login.tmpl")
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
		Head: h.headHtml,
	}

	w.WriteHeader(code)
	loginTemplate.Execute(w, loginData)
}

func (h *WebUiHandler) users(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		r.ParseForm()

		if len(r.Form["username"]) != 1 {
			w.WriteHeader(400)
			w.Write([]byte("Invalid username parameter"))
			return
		}
		username := r.Form["username"][0]

		minUsernameLen := 6
		if len(username) < minUsernameLen {
			w.WriteHeader(400)
			errStr := fmt.Sprintf("Username must be at least %d characters", minUsernameLen)
			h.alertDialog(w, r, errStr, "/users")
			return
		}

		isAdmin := len(r.Form["is-admin"]) == 1 && r.Form["is-admin"][0] == "on"

		err := h.db.AddUser(username, isAdmin)
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, err.Error(), "/users")
			return
		}

		http.Redirect(w, r, "/users", 303)
	}

	tmpl, err := h.loadTemplate("users.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
		return
	}

	tmpl.Execute(w, UsersData{
		Head:  h.headHtml,
		Users: h.db.GetUsers(),
	})
}

func (h *WebUiHandler) confirmDeleteUser(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	if len(r.Form["username"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid username parameter"))
		return
	}
	username := r.Form["username"][0]

	tmpl, err := h.loadTemplate("confirm.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
		return
	}

	data := &ConfirmData{
		Head:       h.headHtml,
		Message:    fmt.Sprintf("Are you sure you want to delete user %s?", username),
		ConfirmUrl: fmt.Sprintf("/delete-user?username=%s", username),
		CancelUrl:  "/users",
	}

	tmpl.Execute(w, data)
}

func (h *WebUiHandler) deleteUser(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	if len(r.Form["username"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid username parameter"))
		return
	}
	username := r.Form["username"][0]

	h.db.DeleteUser(username)

	http.Redirect(w, r, "/users", 303)
}

func (h *WebUiHandler) alertDialog(w http.ResponseWriter, r *http.Request, message, redirectUrl string) error {
	tmpl, err := h.loadTemplate("alert.tmpl")
	if err != nil {
		return err
	}

	tmpl.Execute(w, &AlertData{
		Head:        h.headHtml,
		Message:     message,
		RedirectUrl: redirectUrl,
	})

	return nil
}

func (h *WebUiHandler) loadTemplate(name string) (*template.Template, error) {

	tmplStr, err := h.box.String(name)
	if err != nil {
		return nil, errors.New("Error reading template " + name)
	}

	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return nil, errors.New("Error compiling template " + name)
	}

	return tmpl, nil
}
