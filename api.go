package main

import (
        //"fmt"
        //"strings"
        "net/http"
        "io"
        "encoding/json"
)

type Api struct {
	config        *BoringProxyConfig
        auth *Auth
        tunMan *TunnelManager
        mux *http.ServeMux
}

type CreateTunnelResponse struct {
        ServerAddress string `json:"server_address"`
        ServerPort int `json:"server_port"`
        ServerPublicKey string `json:"server_public_key"`
        TunnelPort int `json:"tunnel_port"`
        TunnelPrivateKey string `json:"tunnel_private_key"`
}


func NewApi(config *BoringProxyConfig, auth *Auth, tunMan *TunnelManager) *Api {

        api := &Api{config, auth, tunMan, nil}

        mux := http.NewServeMux()

        mux.Handle("/tunnels", http.StripPrefix("/tunnels", http.HandlerFunc(api.handleTunnels)))

        api.mux = mux

        return api
}


func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        a.mux.ServeHTTP(w, r)
}

func (a *Api) handleTunnels(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case "POST":
                a.validateSession(http.HandlerFunc(a.handleCreateTunnel)).ServeHTTP(w, r)
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

        port, privKey, err := a.tunMan.CreateTunnel(domain)
        if err != nil {
		w.WriteHeader(400)
                io.WriteString(w, err.Error())
		return
        }

        response := &CreateTunnelResponse{
                ServerAddress: a.config.AdminDomain,
                ServerPort: 22,
                ServerPublicKey: "",
                TunnelPort: port,
                TunnelPrivateKey: privKey,
        }

        responseJson, err := json.MarshalIndent(response, "", "  ")
        if err != nil {
		w.WriteHeader(500)
                io.WriteString(w, "Error encoding response")
		return
        }

        w.Write(responseJson)
}

func (a *Api) validateSession(h http.Handler) http.Handler {

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
