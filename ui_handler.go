package main

import (
	"errors"
	"fmt"
	"github.com/GeertJohan/go.rice"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
)

type WebUiHandler struct {
	config   *BoringProxyConfig
	db       *Database
	api      *Api
	auth     *Auth
	tunMan   *TunnelManager
	box      *rice.Box
	headHtml template.HTML
	menuHtml template.HTML
}

type TunnelsData struct {
	Head    template.HTML
	Menu    template.HTML
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

type MenuData struct {
	IsAdmin bool
}

type UsersData struct {
	Head  template.HTML
	Menu  template.HTML
	Users map[string]User
}

type TokensData struct {
	Head   template.HTML
	Menu   template.HTML
	Tokens map[string]TokenData
	Users  map[string]User
}

func NewWebUiHandler(config *BoringProxyConfig, db *Database, api *Api, auth *Auth, tunMan *TunnelManager) *WebUiHandler {
	return &WebUiHandler{
		config: config,
		db:     db,
		api:    api,
		auth:   auth,
		tunMan: tunMan,
	}
}

func (h *WebUiHandler) handleWebUiRequest(w http.ResponseWriter, r *http.Request) {

	token, err := extractToken("access_token", r)
	if err != nil {
		h.sendLoginPage(w, r, 401)
		return
	}

	tokenData, exists := h.db.GetTokenData(token)
	if !exists {
		h.sendLoginPage(w, r, 403)
		return
	}

	user, _ := h.db.GetUser(tokenData.Owner)

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

	menuTmplStr, err := box.String("menu.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Error loading menu.tmpl")
		return
	}

	menuTmpl, err := template.New("menu").Parse(menuTmplStr)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, "Failed to parse menu.tmpl", "/")
		return
	}

	var menuBuilder strings.Builder
	menuTmpl.Execute(&menuBuilder, MenuData{IsAdmin: user.IsAdmin})

	h.menuHtml = template.HTML(menuBuilder.String())

	switch r.URL.Path {
	case "/login":
		h.handleLogin(w, r)
	case "/users":
		if user.IsAdmin {
			h.usersPage(w, r)
		} else {
			w.WriteHeader(403)
			h.alertDialog(w, r, "Not authorized", "/")
			return
		}

	case "/confirm-delete-user":
		h.confirmDeleteUser(w, r)
	case "/delete-user":
		if user.IsAdmin {
			h.deleteUser(w, r)
		} else {
			w.WriteHeader(403)
			h.alertDialog(w, r, "Not authorized", "/")
			return
		}
	case "/":
		http.Redirect(w, r, "/tunnels", 302)
	case "/tunnels":
		h.handleTunnels(w, r, tokenData)
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

	case "/tokens":
		h.tokensPage(w, r, user, tokenData)
	case "/confirm-delete-token":
		h.confirmDeleteToken(w, r)
	case "/delete-token":
		h.deleteToken(w, r)

	default:
		w.WriteHeader(404)
		h.alertDialog(w, r, "Unknown page "+r.URL.Path, "/")
		return
	}
}

func (h *WebUiHandler) tokensPage(w http.ResponseWriter, r *http.Request, user User, tokenData TokenData) {

	if r.Method == "POST" {
		r.ParseForm()

		if len(r.Form["owner"]) != 1 {
			w.WriteHeader(400)
			h.alertDialog(w, r, "Invalid owner parameter", "/tokens")
			return
		}
		owner := r.Form["owner"][0]

		users := h.db.GetUsers()

		_, exists := users[owner]
		if !exists {
			w.WriteHeader(400)
			h.alertDialog(w, r, "Owner doesn't exist", "/tokens")
			return
		}

		_, err := h.db.AddToken(owner)
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, "Failed creating token", "/tokens")
			return
		}

		http.Redirect(w, r, "/tokens", 303)
	}

	tmpl, err := h.loadTemplate("tokens.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
		return
	}

	var tokens map[string]TokenData
	var users map[string]User

	if user.IsAdmin {
		tokens = h.db.GetTokens()
		users = h.db.GetUsers()
	} else {
		tokens = make(map[string]TokenData)

		for token, td := range h.db.GetTokens() {
			if tokenData.Owner == td.Owner {
				tokens[token] = td
			}
		}

		users = make(map[string]User)
		users[tokenData.Owner] = user
	}

	tmpl.Execute(w, TokensData{
		Head:   h.headHtml,
		Menu:   h.menuHtml,
		Tokens: tokens,
		Users:  users,
	})
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

func (h *WebUiHandler) handleTunnels(w http.ResponseWriter, r *http.Request, tokenData TokenData) {

	switch r.Method {
	case "GET":
		tunnelsTemplate, err := h.box.String("tunnels.tmpl")
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, "Error reading tunnels.tmpl")
			return
		}

		tmpl, err := template.New("tunnels").Parse(tunnelsTemplate)
		if err != nil {
			w.WriteHeader(500)
			log.Println(err)
			io.WriteString(w, "Error compiling tunnels.tmpl")
			return
		}

		tunnelsData := TunnelsData{
			Head:    h.headHtml,
			Menu:    h.menuHtml,
			Tunnels: h.api.GetTunnels(tokenData),
		}

		tmpl.Execute(w, tunnelsData)
	case "POST":
		h.handleCreateTunnel(w, r, tokenData)
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /tunnels"))
		return
	}
}

func (h *WebUiHandler) handleCreateTunnel(w http.ResponseWriter, r *http.Request, tokenData TokenData) {

	r.ParseForm()

	_, err := h.api.CreateTunnel(tokenData, r.Form)
	if err != nil {
		w.WriteHeader(400)
		h.alertDialog(w, r, err.Error(), "/")
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

func (h *WebUiHandler) usersPage(w http.ResponseWriter, r *http.Request) {

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
		Menu:  h.menuHtml,
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

func (h *WebUiHandler) confirmDeleteToken(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	if len(r.Form["token"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid token parameter"))
		return
	}
	token := r.Form["token"][0]

	tmpl, err := h.loadTemplate("confirm.tmpl")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, err.Error())
		return
	}

	data := &ConfirmData{
		Head:       h.headHtml,
		Message:    fmt.Sprintf("Are you sure you want to delete token %s?", token),
		ConfirmUrl: fmt.Sprintf("/delete-token?token=%s", token),
		CancelUrl:  "/tokens",
	}

	tmpl.Execute(w, data)
}

func (h *WebUiHandler) deleteToken(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if len(r.Form["token"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid token parameter"))
		return
	}
	token := r.Form["token"][0]

	h.db.DeleteTokenData(token)

	http.Redirect(w, r, "/tokens", 303)

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
