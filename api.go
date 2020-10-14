package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Api struct {
	config *BoringProxyConfig
	db     *Database
	auth   *Auth
	tunMan *TunnelManager
	mux    *http.ServeMux
}

func NewApi(config *BoringProxyConfig, db *Database, auth *Auth, tunMan *TunnelManager) *Api {

	mux := http.NewServeMux()

	api := &Api{config, db, auth, tunMan, mux}

	mux.Handle("/tunnels", http.StripPrefix("/tunnels", http.HandlerFunc(api.handleTunnels)))

	return api
}

func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *Api) GetTunnels(tokenData TokenData) map[string]Tunnel {

	user, _ := a.db.GetUser(tokenData.Owner)

	var tunnels map[string]Tunnel

	if user.IsAdmin {
		tunnels = a.db.GetTunnels()
	} else {
		tunnels = make(map[string]Tunnel)

		for domain, tun := range a.db.GetTunnels() {
			if tokenData.Owner == tun.Owner {
				tunnels[domain] = tun
			}
		}
	}

	return tunnels
}

func (a *Api) CreateTunnel(tokenData TokenData, params url.Values) (*Tunnel, error) {

	if len(params["domain"]) != 1 {
		return nil, errors.New("Invalid domain parameter")
	}
	domain := params["domain"][0]

	if len(params["client-name"]) != 1 {
		return nil, errors.New("Invalid client-name parameter")
	}
	clientName := params["client-name"][0]

	if len(params["client-port"]) != 1 {
		return nil, errors.New("Invalid client-port parameter")
	}

	clientPort, err := strconv.Atoi(params["client-port"][0])
	if err != nil {
		return nil, errors.New("Invalid client-port parameter")
	}

	tunnel, err := a.tunMan.CreateTunnelForClient(domain, tokenData.Owner, clientName, clientPort)
	if err != nil {
		return nil, err
	}

	return &tunnel, nil
}

func (a *Api) handleTunnels(w http.ResponseWriter, r *http.Request) {

	token, err := extractToken("access_token", r)
	if err != nil {
		w.WriteHeader(401)
		w.Write([]byte("No token provided"))
		return
	}

	tokenData, exists := a.db.GetTokenData(token)
	if !exists {
		w.WriteHeader(403)
		w.Write([]byte("Not authorized"))
		return
	}

	switch r.Method {
	case "GET":
		query := r.URL.Query()

		tunnels := a.GetTunnels(tokenData)

		if len(query["client-name"]) == 1 {
			clientName := query["client-name"][0]
			for k, tun := range tunnels {
				if tun.ClientName != clientName {
					delete(tunnels, k)
				}
			}
		}

		body, err := json.Marshal(tunnels)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte("Error encoding tunnels"))
			return
		}

		hash := md5.Sum(body)
		hashStr := fmt.Sprintf("%x", hash)

		w.Header()["ETag"] = []string{hashStr}

		w.Write([]byte(body))
	case "POST":
		a.handleCreateTunnel(w, r)
	case "DELETE":
		a.handleDeleteTunnel(w, r)
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /tunnels"))
	}
}

func (a *Api) handleCreateTunnel(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	if len(query["domain"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid domain parameter"))
		return
	}
	domain := query["domain"][0]

	if len(query["owner"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid owner parameter"))
		return
	}
	owner := query["owner"][0]

	tunnel, err := a.tunMan.CreateTunnel(domain, owner)
	if err != nil {
		w.WriteHeader(400)
		io.WriteString(w, err.Error())
		return
	}

	tunnelJson, err := json.MarshalIndent(tunnel, "", "  ")
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Error encoding tunnel")
		return
	}

	w.Write(tunnelJson)
}

func (a *Api) handleDeleteTunnel(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	if len(query["domain"]) != 1 {
		w.WriteHeader(400)
		w.Write([]byte("Invalid domain parameter"))
		return
	}
	domain := query["domain"][0]

	err := a.tunMan.DeleteTunnel(domain)
	if err != nil {
		w.WriteHeader(500)
		io.WriteString(w, "Failed to delete tunnel")
		return
	}
}

func (a *Api) validateToken(h http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractToken("access_token", r)
		if err != nil {
			w.WriteHeader(401)
			w.Write([]byte("No token provided"))
			return
		}

		if !a.auth.Authorized(token) {
			w.WriteHeader(403)
			w.Write([]byte("Not authorized"))
			return
		}

		h.ServeHTTP(w, r)
	})
}
