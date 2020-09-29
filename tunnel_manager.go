package main

import (
        "log"
        "errors"
        "sync"
        "encoding/json"
        "io/ioutil"
        "github.com/caddyserver/certmagic"
)


type Tunnel struct {
        Port int `json:"port"`
}

type Tunnels map[string]*Tunnel

func NewTunnels() Tunnels {
        return make(map[string]*Tunnel)
}

type TunnelManager struct {
        tunnels Tunnels
        mutex *sync.Mutex
        certConfig *certmagic.Config
}

func NewTunnelManager(certConfig *certmagic.Config) *TunnelManager {

        tunnelsJson, err := ioutil.ReadFile("tunnels.json")
        if err != nil {
                log.Println("failed reading tunnels.json")
                tunnelsJson = []byte("{}")
        }

        var tunnels Tunnels

        err = json.Unmarshal(tunnelsJson, &tunnels)
        if err != nil {
                log.Println(err)
                tunnels = NewTunnels()
        }

        for domainName := range tunnels {
                err = certConfig.ManageSync([]string{domainName})
                if err != nil {
                        log.Println("CertMagic error at startup")
                        log.Println(err)
                }
        }

        mutex := &sync.Mutex{}
        return &TunnelManager{tunnels, mutex, certConfig}
}

func (m *TunnelManager) SetTunnel(host string, port int) {
        err := m.certConfig.ManageSync([]string{host})
        if err != nil {
                log.Println("CertMagic error")
                log.Println(err)
        }

        tunnel := &Tunnel{port}
        m.mutex.Lock()
        m.tunnels[host] = tunnel
        saveJson(m.tunnels, "tunnels.json")
        m.mutex.Unlock()
}

func (m *TunnelManager) DeleteTunnel(host string) {
        m.mutex.Lock()
        delete(m.tunnels, host)
        saveJson(m.tunnels, "tunnels.json")
        m.mutex.Unlock()
}

func (m *TunnelManager) GetPort(serverName string) (int, error) {
        m.mutex.Lock()
        tunnel, exists := m.tunnels[serverName]
        m.mutex.Unlock()

        if !exists {
                return 0, errors.New("Doesn't exist")
        }

        return tunnel.Port, nil
}
