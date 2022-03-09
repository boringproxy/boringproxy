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
	AdminDomain   string                         `json:"admin_domain"`
	Tokens        map[string]TokenData           `json:"tokens"`
	Tunnels       map[string]Tunnel              `json:"tunnels"`
	Users         map[string]User                `json:"users"`
	Domains       map[string]Domain              `json:"domains"`
	dnsRequests   map[string]namedrop.DNSRequest `json:"dns_requests"`
	Waygates      map[string]waygate.Waygate     `json:"waygates"`
	WaygateTokens map[string]waygate.TokenData   `json:"waygate_tokens"`
	waygateCodes  map[string]string              `json:"waygate_codes"`
	mutex         *sync.Mutex
}

type TokenData struct {
	Owner  string `json:"owner"`
	Client string `json:"client,omitempty"`
}

type User struct {
	IsAdmin bool                `json:"is_admin"`
	Clients map[string]DbClient `json:"clients"`
}

type Domain struct {
	Owner string `json:"owner"`
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

	if db.Domains == nil {
		db.Domains = make(map[string]Domain)
	}

	if db.dnsRequests == nil {
		db.dnsRequests = make(map[string]namedrop.DNSRequest)
	}

	if db.Waygates == nil {
		db.Waygates = make(map[string]waygate.Waygate)
	}
	if db.WaygateTokens == nil {
		db.WaygateTokens = make(map[string]waygate.TokenData)
	}
	if db.waygateCodes == nil {
		db.waygateCodes = make(map[string]string)
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

func (d *Database) GetLegacyTokenData(token string) (TokenData, bool) {
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

func (d *Database) GetDomains() map[string]Domain {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	domains := make(map[string]Domain)

	for k, v := range d.Domains {
		domains[k] = v
	}

	return domains
}

func (d *Database) AddDomain(domain, owner string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.Domains[domain]

	if exists {
		return errors.New("Domain already taken")
	}

	_, exists = d.Users[owner]
	if !exists {
		return errors.New("No such user")
	}

	d.Domains[domain] = Domain{
		Owner: owner,
	}

	d.persist()

	return nil
}
func (d *Database) DeleteDomain(domain string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	delete(d.Domains, domain)

	d.persist()
}

func (d *Database) AddWaygate(domains []string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	id, err := genRandomCode(32)
	if err != nil {
		return "", errors.New("Could not generate waygate id")
	}

	for _, domainName := range domains {
		for _, waygate := range d.Waygates {
			for _, waygateDomainName := range waygate.Domains {
				if domainName == waygateDomainName {
					return "", errors.New("Domain already used by another waygate")
				}
			}
		}
	}

	waygate := waygate.Waygate{
		Domains: domains,
	}

	d.Waygates[id] = waygate

	d.persist()

	return id, nil
}
func (d *Database) GetWaygate(id string) (waygate.Waygate, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tun, exists := d.Waygates[id]
	if !exists {
		return waygate.Waygate{}, errors.New("No such waygate")
	}

	return tun, nil
}
func (d *Database) GetWaygates() map[string]waygate.Waygate {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	wgs := make(map[string]waygate.Waygate)

	for id, wg := range d.Waygates {
		wgs[id] = wg
	}

	return wgs
}

func (d *Database) AddWaygateToken(waygateId string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	token, err := genRandomCode(32)
	if err != nil {
		return "", errors.New("Could not generate waygate token")
	}

	_, exists := d.Waygates[waygateId]
	if !exists {
		return "", errors.New("No such waygate")
	}

	tokenData := waygate.TokenData{
		WaygateId: waygateId,
	}

	d.WaygateTokens[token] = tokenData

	d.persist()

	return token, nil
}
func (d *Database) GetTokenData(id string) (waygate.TokenData, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	token, exists := d.WaygateTokens[id]
	if !exists {
		return waygate.TokenData{}, errors.New("No such token")
	}

	return token, nil
}

func (d *Database) SetTokenCode(token, code string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, exists := d.WaygateTokens[token]
	if !exists {
		return errors.New("No such token")
	}

	d.waygateCodes[code] = token

	d.persist()

	return nil
}
func (d *Database) GetTokenByCode(code string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	token, exists := d.waygateCodes[code]
	if !exists {
		return "", errors.New("No such code")
	}

	delete(d.waygateCodes, code)

	d.persist()

	return token, nil
}

func (d *Database) persist() {
	saveJson(d, DBFolderPath+"boringproxy_db.json")
}
