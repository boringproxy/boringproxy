package main

import (
	"encoding/base64"
	"fmt"
	"github.com/GeertJohan/go.rice"
	qrcode "github.com/skip2/go-qrcode"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebUiHandler struct {
	config          *BoringProxyConfig
	db              *Database
	api             *Api
	auth            *Auth
	tunMan          *TunnelManager
	box             *rice.Box
	headHtml        template.HTML
	pendingRequests map[string]chan string
	mutex           *sync.Mutex
}

type IndexData struct {
	Head    template.HTML
	Tunnels map[string]Tunnel
	Tokens  map[string]TokenData
	Users   map[string]User
	IsAdmin bool
	QrCodes map[string]template.URL
}

type TunnelsData struct {
	Head    template.HTML
	Tunnels map[string]Tunnel
}

type ConfirmData struct {
	Head       template.HTML
	Message    string
	ConfirmUrl string
	CancelUrl  string
}

type LoadingData struct {
	Head      template.HTML
	TargetUrl string
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

type StylesData struct {
	Tunnels map[string]Tunnel
}

type MenuData struct {
	IsAdmin bool
}

type UsersData struct {
	Head  template.HTML
	Users map[string]User
}

type TokensData struct {
	Head   template.HTML
	Tokens map[string]TokenData
	Users  map[string]User
}

func NewWebUiHandler(config *BoringProxyConfig, db *Database, api *Api, auth *Auth, tunMan *TunnelManager) *WebUiHandler {
	return &WebUiHandler{
		config:          config,
		db:              db,
		api:             api,
		auth:            auth,
		tunMan:          tunMan,
		pendingRequests: make(map[string]chan string),
		mutex:           &sync.Mutex{},
	}
}

func (h *WebUiHandler) handleWebUiRequest(w http.ResponseWriter, r *http.Request) {

	homePath := "/#/tunnel"

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

	stylesTmpl, err := template.New("styles").Parse(stylesText)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, err.Error(), homePath)
		return
	}

	tunnels := h.api.GetTunnels(tokenData)

	for domain, tun := range tunnels {
		// TODO: might yield non-unique names
		tun.CssId = strings.ReplaceAll(domain, ".", "-")
		tunnels[domain] = tun
	}

	var stylesBuilder strings.Builder
	stylesTmpl.Execute(&stylesBuilder, StylesData{Tunnels: tunnels})

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
	headTmpl.Execute(&headBuilder, HeadData{Styles: template.CSS(stylesBuilder.String())})

	h.headHtml = template.HTML(headBuilder.String())

	switch r.URL.Path {
	case "/login":
		h.handleLogin(w, r)
	case "/users":
		if user.IsAdmin {
			h.handleUsers(w, r)
		} else {
			w.WriteHeader(403)
			h.alertDialog(w, r, "Not authorized", "/#/tunnels")
			return
		}

	case "/confirm-delete-user":
		h.confirmDeleteUser(w, r)
	case "/delete-user":
		if user.IsAdmin {
			h.deleteUser(w, r)
		} else {
			w.WriteHeader(403)
			h.alertDialog(w, r, "Not authorized", "/#/tunnels")
			return
		}
	case "/":
		indexTmplStr, err := h.box.String("index.tmpl")
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, "Error reading index.tmpl", "/#/tunnels")
			return
		}

		tmpl, err := template.New("index").Parse(indexTmplStr)
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, "Error compiling index.tmpl", "/#/tunnels")
			return
		}

		var tokens map[string]TokenData
		var users map[string]User
		qrCodes := make(map[string]template.URL)

		if user.IsAdmin {
			tokens = h.db.GetTokens()
			users = h.db.GetUsers()
		} else {
			tokens = make(map[string]TokenData)

			for token, td := range h.db.GetTokens() {
				if tokenData.Owner == td.Owner {
					tokens[token] = td
				}

				loginUrl := fmt.Sprintf("https://%s/login?access_token=%s", h.config.WebUiDomain, token)

				var png []byte
				png, err := qrcode.Encode(loginUrl, qrcode.Medium, 256)
				if err != nil {
					w.WriteHeader(500)
					h.alertDialog(w, r, err.Error(), "/#/tokens")
					return
				}

				data := base64.StdEncoding.EncodeToString(png)
				qrCodes[token] = template.URL("data:image/png;base64," + data)
			}

			users = make(map[string]User)
			users[tokenData.Owner] = user
		}

		indexData := IndexData{
			Head:    h.headHtml,
			Tunnels: tunnels,
			Tokens:  tokens,
			Users:   users,
			IsAdmin: user.IsAdmin,
			QrCodes: qrCodes,
		}

		err = tmpl.Execute(w, indexData)
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, err.Error(), "/#/tokens")
			return
		}

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
			CancelUrl:  "/#/tunnels",
		}

		tmpl.Execute(w, data)

	case "/delete-tunnel":

		r.ParseForm()

		err := h.api.DeleteTunnel(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(400)
			h.alertDialog(w, r, err.Error(), "/#/tunnels")
			return
		}

		//http.Redirect(w, r, "/#/tunnels", 307)

	case "/tokens":
		h.handleTokens(w, r, user, tokenData)
	case "/confirm-delete-token":
		h.confirmDeleteToken(w, r)
	case "/delete-token":
		h.deleteToken(w, r)
	case "/confirm-logout":
		tmpl, err := h.loadTemplate("confirm.tmpl")
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, err.Error(), "/#/tunnels")
			return
		}

		data := &ConfirmData{
			Head:       h.headHtml,
			Message:    "Are you sure you want to log out?",
			ConfirmUrl: "/logout",
			CancelUrl:  "/#/tunnels",
		}

		tmpl.Execute(w, data)

	case "/logout":
		cookie := &http.Cookie{
			Name:     "access_token",
			Value:    "",
			Secure:   true,
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/#/tunnels", 303)
	case "/loading":
		h.handleLoading(w, r)
	default:
		w.WriteHeader(404)
		h.alertDialog(w, r, "Unknown page "+r.URL.Path, "/#/tunnels")
		return
	}
}

func (h *WebUiHandler) handleTokens(w http.ResponseWriter, r *http.Request, user User, tokenData TokenData) {

	if r.Method != "POST" {
		w.WriteHeader(405)
		h.alertDialog(w, r, "Invalid method for tokens", "/#/tokens")
		return
	}

	r.ParseForm()

	if len(r.Form["owner"]) != 1 {
		w.WriteHeader(400)
		h.alertDialog(w, r, "Invalid owner parameter", "/#/tokens")
		return
	}
	owner := r.Form["owner"][0]

	users := h.db.GetUsers()

	_, exists := users[owner]
	if !exists {
		w.WriteHeader(400)
		h.alertDialog(w, r, "Owner doesn't exist", "/#/tokens")
		return
	}

	_, err := h.db.AddToken(owner)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, "Failed creating token", "/#/tokens")
		return
	}

	http.Redirect(w, r, "/#/tokens", 303)
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
		cookie := &http.Cookie{
			Name:     "access_token",
			Value:    token,
			Secure:   true,
			HttpOnly: true,
			MaxAge:   86400 * 365,
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "/#/tunnels", 303)
	} else {
		h.sendLoginPage(w, r, 403)
		return
	}
}

func (h *WebUiHandler) handleTunnels(w http.ResponseWriter, r *http.Request, tokenData TokenData) {

	switch r.Method {
	case "POST":
		h.handleCreateTunnel(w, r, tokenData)
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /#/tunnels"))
		return
	}
}

func (h *WebUiHandler) handleCreateTunnel(w http.ResponseWriter, r *http.Request, tokenData TokenData) {

	pendingId, err := genRandomCode(16)
	if err != nil {
		w.WriteHeader(400)
		h.alertDialog(w, r, err.Error(), "/#/tunnels")
	}

	doneSignal := make(chan string)
	h.mutex.Lock()
	h.pendingRequests[pendingId] = doneSignal
	h.mutex.Unlock()

	go func() {

		r.ParseForm()

		_, err := h.api.CreateTunnel(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(400)
			h.alertDialog(w, r, err.Error(), "/#/tunnels")
		}

		doneSignal <- "/#/tunnels"
	}()

	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		timeout <- true
	}()

	select {
	case <-timeout:
		url := fmt.Sprintf("/loading?id=%s", pendingId)

		tmpl, err := h.loadTemplate("loading.tmpl")
		if err != nil {
			w.WriteHeader(500)
			h.alertDialog(w, r, err.Error(), "/#/tunnels")
			return
		}

		data := &LoadingData{
			Head:      h.headHtml,
			TargetUrl: url,
		}

		tmpl.Execute(w, data)

	case <-doneSignal:
		http.Redirect(w, r, "/#/tunnels", 303)
	}
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

func (h *WebUiHandler) handleUsers(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		w.WriteHeader(405)
		h.alertDialog(w, r, "Invalid method for users", "/#/users")
		return
	}

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
		h.alertDialog(w, r, errStr, "/#/users")
		return
	}

	isAdmin := len(r.Form["is-admin"]) == 1 && r.Form["is-admin"][0] == "on"

	err := h.db.AddUser(username, isAdmin)
	if err != nil {
		w.WriteHeader(500)
		h.alertDialog(w, r, err.Error(), "/#/users")
		return
	}

	http.Redirect(w, r, "/#/users", 303)
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
		CancelUrl:  "/#/users",
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

	http.Redirect(w, r, "/#/users", 303)
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
		CancelUrl:  "/#/tokens",
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

	http.Redirect(w, r, "/#/tokens", 303)

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

func (h *WebUiHandler) handleLoading(w http.ResponseWriter, r *http.Request) {

	if r.Method != "GET" {
		w.WriteHeader(405)
		h.alertDialog(w, r, "Invalid method for users", "/#/tunnels")
	}

	r.ParseForm()

	pendingId := r.Form.Get("id")

	h.mutex.Lock()
	doneSignal := h.pendingRequests[pendingId]
	delete(h.pendingRequests, pendingId)
	h.mutex.Unlock()

	redirUrl := <-doneSignal

	http.Redirect(w, r, redirUrl, 303)
}

func (h *WebUiHandler) loadTemplate(name string) (*template.Template, error) {

	tmplStr, err := h.box.String(name)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}
