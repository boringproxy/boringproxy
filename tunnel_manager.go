package main

import (
        "log"
        "errors"
        "sync"
        "github.com/caddyserver/certmagic"
)


type TunnelManager struct {
        tunnels map[string]int
        mutex *sync.Mutex
        certConfig *certmagic.Config
}

func NewTunnelManager(certConfig *certmagic.Config) *TunnelManager {
        tunnels := make(map[string]int)
        mutex := &sync.Mutex{}
        return &TunnelManager{tunnels, mutex, certConfig}
}

func (m *TunnelManager) SetTunnel(host string, port int) {
        err := m.certConfig.ManageSync([]string{host})
        if err != nil {
                log.Println("CertMagic error")
                log.Println(err)
        }

        m.mutex.Lock()
        m.tunnels[host] = port
        m.mutex.Unlock()
}

func (m *TunnelManager) DeleteTunnel(host string) {
        m.mutex.Lock()
        delete(m.tunnels, host)
        m.mutex.Unlock()
}

func (m *TunnelManager) GetPort(serverName string) (int, error) {
        m.mutex.Lock()
        port, exists := m.tunnels[serverName]
        m.mutex.Unlock()

        if !exists {
                return 0, errors.New("Doesn't exist")
        }

        return port, nil
}
