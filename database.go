package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"sync"
)

type Database struct {
	Sessions map[string]Session `json:"sessions"`
	Tunnels  map[string]Tunnel  `json:"tunnels"`
	mutex    *sync.Mutex
}

type Session struct {
	Id string `json:"id"`
}

type Tunnel struct {
	ServerAddress    string `json:"server_address"`
	ServerPort       int    `json:"server_port"`
	ServerPublicKey  string `json:"server_public_key"`
	Username         string `json:"username"`
	TunnelPort       int    `json:"tunnel_port"`
	TunnelPrivateKey string `json:"tunnel_private_key"`
}


func NewDatabase() (*Database, error) {

	dbJson, err := ioutil.ReadFile("boringproxy_db.json")
	if err != nil {
		log.Println("failed reading boringproxy_db.json")
		dbJson = []byte("{}")
	}

	var db *Database

	err = json.Unmarshal(dbJson, &db)
	if err != nil {
		log.Println(err)
		db = &Database{}
	}

	if db.Sessions == nil {
		db.Sessions = make(map[string]Session)
	}

	if db.Tunnels == nil {
		db.Tunnels = make(map[string]Tunnel)
	}

	db.mutex = &sync.Mutex{}

	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.persist()

	return db, nil
}

func (d *Database) GetSession(token string) (Session, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	sesh, exists := d.Sessions[token]

	if !exists {
		return Session{}, false
	}

	return sesh, true
}

func (d *Database) SetSession(token string, sesh Session) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Sessions[token] = sesh
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

func (d *Database) persist() {
	saveJson(d, "boringproxy_db.json")
}
