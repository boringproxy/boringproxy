package boringproxy

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
	config *Config
	db     *Database
	auth   *Auth
	tunMan *TunnelManager
	mux    *http.ServeMux
}

func NewApi(config *Config, db *Database, auth *Auth, tunMan *TunnelManager) *Api {

	mux := http.NewServeMux()

	api := &Api{config, db, auth, tunMan, mux}

	mux.Handle("/tunnels", http.StripPrefix("/tunnels", http.HandlerFunc(api.handleTunnels)))
	mux.Handle("/users/", http.StripPrefix("/users", http.HandlerFunc(api.handleUsers)))
	mux.Handle("/tokens/", http.StripPrefix("/tokens", http.HandlerFunc(api.handleTokens)))
	mux.Handle("/clients/", http.StripPrefix("/clients", http.HandlerFunc(api.handleClients)))

	return api
}

func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
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

		// If the token is limited to a specific client, filter out
		// tunnels for any other clients.
		if tokenData.Client != "" {
			for k, tun := range tunnels {
				if tokenData.Client != tun.ClientName {
					delete(tunnels, k)
				}
			}
		}

		clientName := query.Get("client-name")
		if clientName != "" && tokenData.Client != "" && clientName != tokenData.Client {
			w.WriteHeader(403)
			w.Write([]byte("Token is not valid for this client"))
			return
		}

		if clientName != "" {
			for k, tun := range tunnels {
				if tun.ClientName != clientName {
					delete(tunnels, k)
				} else {
					tun.ServerPort = a.config.SshServerPort
					tunnels[k] = tun
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

		if tokenData.Client != "" {
			w.WriteHeader(403)
			io.WriteString(w, "Token cannot be used to create tunnels")
			return
		}

		r.ParseForm()
		_, err := a.CreateTunnel(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}
	case "DELETE":
		if tokenData.Client != "" {
			w.WriteHeader(403)
			io.WriteString(w, "Token cannot be used to delete tunnels")
			return
		}

		r.ParseForm()
		err := a.DeleteTunnel(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}
	default:
		w.WriteHeader(405)
		w.Write([]byte("Invalid method for /tunnels"))
	}
}

func (a *Api) handleUsers(w http.ResponseWriter, r *http.Request) {
	token, err := extractToken("access_token", r)
	if err != nil {
		w.WriteHeader(401)
		io.WriteString(w, "Invalid token")
		return
	}

	tokenData, exists := a.db.GetTokenData(token)
	if !exists {
		w.WriteHeader(401)
		io.WriteString(w, "Failed to get token")
		return
	}

	r.ParseForm()

	if tokenData.Client != "" {
		w.WriteHeader(403)
		io.WriteString(w, "Token cannot be used to create users")
		return
	}

	switch r.Method {
	case "POST":
		err := a.CreateUser(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, err.Error())
			return
		}
	default:
		w.WriteHeader(405)
		io.WriteString(w, "Invalid method for /users")
		return
	}
}

func (a *Api) handleTokens(w http.ResponseWriter, r *http.Request) {
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

	if tokenData.Client != "" {
		w.WriteHeader(403)
		io.WriteString(w, "Token cannot be used to manage tokens")
		return
	}

	switch r.Method {
	case "POST":
		r.ParseForm()
		token, err := a.CreateToken(tokenData, r.Form)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}

		io.WriteString(w, token)
	default:
		w.WriteHeader(405)
		w.Write([]byte(err.Error()))
	}
}

func (a *Api) handleClients(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

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

	clientName := r.Form.Get("client-name")
	if clientName == "" {
		if tokenData.Client == "" {
			w.WriteHeader(400)
			w.Write([]byte("Missing client-name parameter"))
			return
		} else {
			clientName = tokenData.Client
		}
	}

	if tokenData.Client != "" && tokenData.Client != clientName {
		w.WriteHeader(403)
		io.WriteString(w, "Token does not have proper permissions")
		return
	}

	user := r.Form.Get("user")
	if user == "" {
		user = tokenData.Owner
	}

	switch r.Method {
	case "POST":
		err := a.SetClient(tokenData, r.Form, user, clientName)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
		}
	case "DELETE":
		err := a.DeleteClient(tokenData, user, clientName)
		if err != nil {
			w.WriteHeader(500)
			io.WriteString(w, err.Error())
			return
		}
	default:
		w.WriteHeader(405)
		w.Write([]byte(err.Error()))
	}
}

func (a *Api) GetTunnel(tokenData TokenData, params url.Values) (Tunnel, error) {
	domain := params.Get("domain")
	if domain == "" {
		return Tunnel{}, errors.New("Invalid domain parameter")
	}

	tun, exists := a.db.GetTunnel(domain)
	if !exists {
		return Tunnel{}, errors.New("Tunnel doesn't exist for domain")
	}

	user, _ := a.db.GetUser(tokenData.Owner)
	if user.IsAdmin || tun.Owner == tokenData.Owner {
		return tun, nil
	} else {
		return Tunnel{}, errors.New("Unauthorized")
	}
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

	domain := params.Get("domain")
	if domain == "" {
		return nil, errors.New("Invalid domain parameter")
	}

	owner := params.Get("owner")
	if owner == "" {
		return nil, errors.New("Invalid owner parameter")
	}

	// Only admins can create tunnels for other users
	if tokenData.Owner != owner {
		user, _ := a.db.GetUser(tokenData.Owner)
		if !user.IsAdmin {
			return nil, errors.New("Unauthorized")
		}
	}

	clientName := params.Get("client-name")

	clientPort := 0
	clientPortParam := params.Get("client-port")
	if clientPortParam != "" {
		var err error
		clientPort, err = strconv.Atoi(clientPortParam)
		if err != nil {
			return nil, errors.New("Invalid client-port parameter")
		}
	}

	clientAddr := params.Get("client-addr")
	if clientAddr == "" {
		clientAddr = "127.0.0.1"
	}

	tunnelPort := 0
	tunnelPortParam := params.Get("tunnel-port")
	if tunnelPortParam != "" && tunnelPortParam != "Random" {
		var err error
		tunnelPort, err = strconv.Atoi(tunnelPortParam)
		if err != nil {
			return nil, errors.New("Invalid tunnel-port parameter")
		}
	}

	allowExternalTcp := params.Get("allow-external-tcp") == "on"

	passwordProtect := params.Get("password-protect") == "on"

	var username string
	var password string
	if passwordProtect {
		username = params.Get("username")
		if username == "" {
			return nil, errors.New("Username required")
		}

		password = params.Get("password")
		if password == "" {
			return nil, errors.New("Password required")
		}
	}

	tlsTerm := params.Get("tls-termination")
	if tlsTerm != "server" && tlsTerm != "client" && tlsTerm != "passthrough" && tlsTerm != "client-tls" && tlsTerm != "server-tls" {
		return nil, errors.New("Invalid tls-termination parameter")
	}

	sshServerAddr := a.db.GetAdminDomain()
	sshServerAddrParam := params.Get("ssh-server-addr")
	if sshServerAddrParam != "" {
		sshServerAddr = sshServerAddrParam
	}

	sshServerPort := a.config.SshServerPort
	sshServerPortParam := params.Get("ssh-server-port")
	if sshServerPortParam != "" {
		var err error
		sshServerPort, err = strconv.Atoi(sshServerPortParam)
		if err != nil {
			return nil, errors.New("Invalid ssh-server-port parameter")
		}
	}

	request := Tunnel{
		Domain:           domain,
		Owner:            owner,
		ClientName:       clientName,
		ClientPort:       clientPort,
		ClientAddress:    clientAddr,
		TunnelPort:       tunnelPort,
		AllowExternalTcp: allowExternalTcp,
		AuthUsername:     username,
		AuthPassword:     password,
		TlsTermination:   tlsTerm,
		ServerAddress:    sshServerAddr,
		ServerPort:       sshServerPort,
	}

	tunnel, err := a.tunMan.RequestCreateTunnel(request)
	if err != nil {
		return nil, err
	}

	return &tunnel, nil
}

func (a *Api) DeleteTunnel(tokenData TokenData, params url.Values) error {

	domain := params.Get("domain")
	if domain == "" {
		return errors.New("Invalid domain parameter")
	}

	tun, exists := a.db.GetTunnel(domain)
	if !exists {
		return errors.New("Tunnel doesn't exist")
	}

	if tokenData.Owner != tun.Owner {
		user, _ := a.db.GetUser(tokenData.Owner)
		if !user.IsAdmin {
			return errors.New("Unauthorized")
		}
	}

	a.tunMan.DeleteTunnel(domain)

	return nil
}

func (a *Api) CreateToken(tokenData TokenData, params url.Values) (string, error) {

	owner := params.Get("owner")
	if owner == "" {
		return "", errors.New("Invalid owner paramater")
	}

	user, _ := a.db.GetUser(tokenData.Owner)

	if tokenData.Owner != owner && !user.IsAdmin {
		return "", errors.New("Unauthorized")
	}

	client := params.Get("client")

	if client != "any" {
		if _, exists := user.Clients[client]; !exists {
			return "", errors.New(fmt.Sprintf("Client %s does not exist for user %s", client, owner))
		}
	} else {
		client = ""
	}

	token, err := a.db.AddToken(owner, client)
	if err != nil {
		return "", errors.New("Failed to create token")
	}

	return token, nil
}

func (a *Api) DeleteToken(tokenData TokenData, params url.Values) error {
	token := params.Get("token")
	if token == "" {
		return errors.New("Invalid token parameter")
	}

	delTokenData, exists := a.db.GetTokenData(token)
	if !exists {
		return errors.New("Token doesn't exist")
	}

	if tokenData.Owner != delTokenData.Owner {
		user, _ := a.db.GetUser(tokenData.Owner)
		if !user.IsAdmin {
			return errors.New("Unauthorized")
		}
	}

	a.db.DeleteTokenData(token)

	return nil

}

func (a *Api) CreateUser(tokenData TokenData, params url.Values) error {

	user, _ := a.db.GetUser(tokenData.Owner)
	if !user.IsAdmin {
		return errors.New("Unauthorized")
	}

	username := params.Get("username")
	minUsernameLen := 6
	if len(username) < minUsernameLen {
		errStr := fmt.Sprintf("Username must be at least %d characters", minUsernameLen)
		return errors.New(errStr)
	}

	isAdmin := params.Get("is-admin") == "on"

	err := a.db.AddUser(username, isAdmin)
	if err != nil {
		return err
	}

	return nil
}

func (a *Api) DeleteUser(tokenData TokenData, params url.Values) error {

	user, _ := a.db.GetUser(tokenData.Owner)
	if !user.IsAdmin {
		return errors.New("Unauthorized")
	}

	username := params.Get("username")
	if username == "" {
		return errors.New("Invalid username parameter")
	}

	_, exists := a.db.GetUser(username)
	if !exists {
		return errors.New("User doesn't exist")
	}

	a.db.DeleteUser(username)

	for token, tokenData := range a.db.GetTokens() {
		if tokenData.Owner == username {
			a.db.DeleteTokenData(token)
		}
	}

	return nil
}

func (a *Api) SetClient(tokenData TokenData, params url.Values, ownerId, clientId string) error {

	if tokenData.Owner != ownerId {
		user, _ := a.db.GetUser(tokenData.Owner)
		if !user.IsAdmin {
			return errors.New("Unauthorized")
		}
	}

	// TODO: what if two users try to get then set at the same time?
	owner, _ := a.db.GetUser(ownerId)
	owner.Clients[clientId] = DbClient{}
	a.db.SetUser(ownerId, owner)

	return nil
}

func (a *Api) DeleteClient(tokenData TokenData, ownerId, clientId string) error {

	if tokenData.Owner != ownerId {
		user, _ := a.db.GetUser(tokenData.Owner)
		if !user.IsAdmin {
			return errors.New("Unauthorized")
		}
	}

	owner, _ := a.db.GetUser(ownerId)
	delete(owner.Clients, clientId)
	a.db.SetUser(ownerId, owner)

	return nil
}
