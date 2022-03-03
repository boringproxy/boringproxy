package boringproxy

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"sync"

	"github.com/takingnames/namedrop-go"
	"github.com/takingnames/waygate-go"
)

var DBFolderPath string

type Database struct {
	AdminDomain          string                             `json:"admin_domain"`
	Tokens               map[string]TokenData               `json:"tokens"`
	Tunnels              map[string]Tunnel                  `json:"tunnels"`
	Users                map[string]User                    `json:"users"`
	dnsRequests          map[string]namedrop.DNSRequest     `json:"dns_requests"`
	WaygateTunnels       map[string]waygate.WaygateTunnel   `json:"waygate_tunnels"`
	WaygateTalismans     map[string]waygate.WaygateTalisman `json:"waygate_talismans"`
	WaygatePendingTokens map[string]string                  `json:"waygate_pending_tokens"`
	mutex                *sync.Mutex
}

type TokenData struct {
	Owner  string `json:"owner"`
	Client string `json:"client,omitempty"`
}

type User struct {
	IsAdmin bool                `json:"is_admin"`
	Clients map[string]DbClient `json:"clients"`
}

type DbClient struct {
}

type DNSRecord struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
}

type Tunnel struct {
	Domain           string `json:"domain"`
	ServerAddress    string `json:"server_address"`
	ServerPort       int    `json:"server_port"`
	ServerPublicKey  string `json:"server_public_key"`
	Username         string `json:"username"`
	TunnelPort       int    `json:"tunnel_port"`
	TunnelPrivateKey string `json:"tunnel_private_key"`
	ClientAddress    string `json:"client_address"`
	ClientPort       int    `json:"client_port"`
	AllowExternalTcp bool   `json:"allow_external_tcp"`
	TlsTermination   string `json:"tls_termination"`

	// TODO: These are not used by clients and possibly shouldn't be
	// returned in API calls.
	Owner        string `json:"owner"`
	ClientName   string `json:"client_name"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
}

func NewDatabase(path string) (*Database, error) {

	DBFolderPath = path

	dbJson, err := ioutil.ReadFile(DBFolderPath + "boringproxy_db.json")
	if err != nil {
		log.Printf("failed reading %sboringproxy_db.json\n", DBFolderPath)
		dbJson = []byte("{}")
	}

	var db *Database

	err = json.Unmarshal(dbJson, &db)
	if err != nil {
		log.Println(err)
		db = &Database{}
	}

	if db.Tokens == nil {
		db.Tokens = make(map[string]TokenData)
	}

	if db.Tunnels == nil {
		db.Tunnels = make(map[string]Tunnel)
	}

	if db.Users == nil {
		db.Users = make(map[string]User)
	}

	if db.dnsRequests == nil {
		db.dnsRequests = make(map[string]namedrop.DNSRequest)
	}

	if db.WaygateTunnels == nil {
		db.WaygateTunnels = make(map[string]waygate.WaygateTunnel)
	}
	if db.WaygateTalismans == nil {
		db.WaygateTalismans = make(map[string]waygate.WaygateTalisman)
	}
	if db.WaygatePendingTokens == nil {
		db.WaygatePendingTokens = make(map[string]string)
	}

	db.mutex = &sync.Mutex{}

	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.persist()

	return db, nil
}

func (d *Database) SetAdminDomain(adminDomain string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.AdminDomain = adminDomain

	d.persist()
}
func (d *Database) GetAdminDomain() string {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.AdminDomain
}

func (d *Database) SetDNSRequest(requestId string, request namedrop.DNSRequest) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.dnsRequests[requestId] = request

	// Not currently persisting because dnsRequests is only stored in
	// memory. May change in the future.
	//d.persist()
}
func (d *Database) GetDNSRequest(requestId string) (namedrop.DNSRequest, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if req, ok := d.dnsRequests[requestId]; ok {
		return req, nil
	}

	return namedrop.DNSRequest{}, errors.New("No such DNS Request")
}
func (d *Database) DeleteDNSRequest(requestId string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.dnsRequests, requestId)
}

func (d *Database) AddToken(owner, client string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.Users[owner]
	if !exists {
		return "", errors.New("Owner doesn't exist")
	}

	token, err := genRandomCode(32)
	if err != nil {
		return "", errors.New("Could not generat token")
	}

	d.Tokens[token] = TokenData{
		Owner:  owner,
		Client: client,
	}

	d.persist()

	return token, nil
}

func (d *Database) GetTokens() map[string]TokenData {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tokens := make(map[string]TokenData)

	for k, v := range d.Tokens {
		tokens[k] = v
	}

	return tokens
}

func (d *Database) GetTokenData(token string) (TokenData, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tokenData, exists := d.Tokens[token]

	if !exists {
		return TokenData{}, false
	}

	return tokenData, true
}

func (d *Database) SetTokenData(token string, tokenData TokenData) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Tokens[token] = tokenData
	d.persist()
}

func (d *Database) DeleteTokenData(token string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.Tokens, token)

	d.persist()
}

func (d *Database) GetTunnels() map[string]Tunnel {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tunnels := make(map[string]Tunnel)

	for k, v := range d.Tunnels {
		tunnels[k] = v
	}

	return tunnels
}

func (d *Database) GetTunnel(domain string) (Tunnel, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tun, exists := d.Tunnels[domain]

	if !exists {
		return Tunnel{}, false
	}

	return tun, true
}

func (d *Database) SetTunnel(domain string, tun Tunnel) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Tunnels[domain] = tun
	d.persist()
}

func (d *Database) DeleteTunnel(domain string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.Tunnels, domain)

	d.persist()
}

func (d *Database) GetUsers() map[string]User {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	users := make(map[string]User)

	for k, v := range d.Users {
		users[k] = v
	}

	return users
}

func (d *Database) GetUser(username string) (User, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	user, exists := d.Users[username]

	if !exists {
		return User{}, false
	}

	return user, true
}

func (d *Database) SetUser(username string, user User) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Users[username] = user
	d.persist()

	return nil
}

func (d *Database) AddUser(username string, isAdmin bool) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.Users[username]

	if exists {
		return errors.New("User exists")
	}

	d.Users[username] = User{
		IsAdmin: isAdmin,
		Clients: make(map[string]DbClient),
	}

	d.persist()

	return nil
}

func (d *Database) DeleteUser(username string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.Users, username)

	d.persist()
}

func (d *Database) AddWaygateTunnel(domains []string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	id, err := genRandomCode(32)
	if err != nil {
		return "", errors.New("Could not generate waygate id")
	}

	// TODO: verify none of the domains are already in use.

	waygate := waygate.WaygateTunnel{
		Domains: domains,
	}

	d.WaygateTunnels[id] = waygate

	d.persist()

	return id, nil
}
func (d *Database) GetWaygateTunnel(id string) (waygate.WaygateTunnel, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tun, exists := d.WaygateTunnels[id]
	if !exists {
		return waygate.WaygateTunnel{}, errors.New("No such waygate")
	}

	return tun, nil
}
func (d *Database) GetWaygates() map[string]waygate.WaygateTunnel {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	wgs := make(map[string]waygate.WaygateTunnel)

	for id, wg := range d.WaygateTunnels {
		wgs[id] = wg
	}

	return wgs
}

func (d *Database) AddWaygateTalisman(waygateId string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	talisman, err := genRandomCode(32)
	if err != nil {
		return "", errors.New("Could not generate waygate talisman")
	}

	_, exists := d.WaygateTunnels[waygateId]
	if !exists {
		return "", errors.New("No such waygate")
	}

	talismanData := waygate.WaygateTalisman{
		WaygateId: waygateId,
	}

	d.WaygateTalismans[talisman] = talismanData

	d.persist()

	return talisman, nil
}
func (d *Database) GetWaygateTalisman(id string) (waygate.WaygateTalisman, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	talisman, exists := d.WaygateTalismans[id]
	if !exists {
		return waygate.WaygateTalisman{}, errors.New("No such talisman")
	}

	return talisman, nil
}

func (d *Database) SetTokenCode(token, code string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.WaygateTalismans[token]
	if !exists {
		return errors.New("No such token")
	}

	d.WaygatePendingTokens[code] = token

	d.persist()

	return nil
}
func (d *Database) GetTokenByCode(code string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	token, exists := d.WaygatePendingTokens[code]
	if !exists {
		return "", errors.New("No such code")
	}

	return token, nil
}

func (d *Database) persist() {
	saveJson(d, DBFolderPath+"boringproxy_db.json")
}
