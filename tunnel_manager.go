package boringproxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/caddyserver/certmagic"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"strings"
	"sync"
)

type TunnelManager struct {
	config     *Config
	db         *Database
	mutex      *sync.Mutex
	certConfig *certmagic.Config
	user       *user.User
}

func NewTunnelManager(config *Config, db *Database, certConfig *certmagic.Config) *TunnelManager {

	user, err := user.Current()
	if err != nil {
		log.Fatalf("Unable to get current user: %v", err)
	}

	if config.autoCerts {
		for domainName, tun := range db.GetTunnels() {
			if tun.TlsTermination == "server" || tun.TlsTermination == "server-tls" {
				err = certConfig.ManageSync(context.Background(), []string{domainName})
				if err != nil {
					log.Println("CertMagic error at startup")
					log.Println(err)
				}
			}
		}
	}

	mutex := &sync.Mutex{}
	return &TunnelManager{config, db, mutex, certConfig, user}
}

func (m *TunnelManager) GetTunnels() map[string]Tunnel {
	return m.db.GetTunnels()
}

func (m *TunnelManager) RequestCreateTunnel(tunReq Tunnel) (Tunnel, error) {

	if tunReq.Domain == "" {
		return Tunnel{}, errors.New("Domain required")
	}

	if tunReq.Owner == "" {
		return Tunnel{}, errors.New("Owner required")
	}

	if tunReq.TlsTermination == "server" || tunReq.TlsTermination == "server-tls" {
		if m.config.autoCerts {
			err := m.certConfig.ManageSync(context.Background(), []string{tunReq.Domain})
			if err != nil {
				return Tunnel{}, errors.New("Failed to get cert")
			}
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if tunReq.TunnelPort == 0 {
		var err error
		tunReq.TunnelPort, err = randomOpenPort()
		if err != nil {
			return Tunnel{}, err
		}
	}

	for _, tun := range m.db.GetTunnels() {
		if tunReq.Domain == tun.Domain {
			return Tunnel{}, errors.New("Tunnel domain already in use")
		}

		if tunReq.TunnelPort == tun.TunnelPort {
			return Tunnel{}, errors.New("Tunnel port already in use")
		}
	}

	privKey, err := m.addToAuthorizedKeys(tunReq.Domain, tunReq.TunnelPort, tunReq.AllowExternalTcp)
	if err != nil {
		return Tunnel{}, err
	}

	tunReq.ServerPublicKey = ""
	tunReq.Username = m.user.Username
	tunReq.TunnelPrivateKey = privKey

	m.db.SetTunnel(tunReq.Domain, tunReq)

	return tunReq, nil
}

func (m *TunnelManager) DeleteTunnel(domain string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	tunnel, exists := m.db.GetTunnel(domain)
	if !exists {
		return errors.New("Tunnel doesn't exist")
	}

	m.db.DeleteTunnel(domain)

	authKeysPath := fmt.Sprintf("%s/.ssh/authorized_keys", m.user.HomeDir)

	akBytes, err := ioutil.ReadFile(authKeysPath)
	if err != nil {
		return err
	}

	akStr := string(akBytes)

	lines := strings.Split(akStr, "\n")

	tunnelId := fmt.Sprintf("boringproxy-%s-%d", domain, tunnel.TunnelPort)

	outLines := []string{}

	for _, line := range lines {
		if strings.Contains(line, tunnelId) {
			continue
		}

		outLines = append(outLines, line)
	}

	outStr := strings.Join(outLines, "\n")

	err = ioutil.WriteFile(authKeysPath, []byte(outStr), 0600)
	if err != nil {
		return err
	}

	return nil
}

func (m *TunnelManager) GetPort(domain string) (int, error) {
	tunnel, exists := m.db.GetTunnel(domain)

	if !exists {
		return 0, errors.New("Doesn't exist")
	}

	return tunnel.TunnelPort, nil
}

func (m *TunnelManager) addToAuthorizedKeys(domain string, port int, allowExternalTcp bool) (string, error) {

	authKeysPath := fmt.Sprintf("%s/.ssh/authorized_keys", m.user.HomeDir)

	akFile, err := os.OpenFile(authKeysPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return "", err
	}
	defer akFile.Close()

	akBytes, err := ioutil.ReadAll(akFile)
	if err != nil {
		return "", err
	}

	akStr := string(akBytes)

	var privKey string
	var pubKey string

	pubKey, privKey, err = MakeSSHKeyPair()
	if err != nil {
		return "", err
	}

	pubKey = strings.TrimSpace(pubKey)

	bindAddr := "127.0.0.1"
	if allowExternalTcp {
		bindAddr = "0.0.0.0"
	}

	options := fmt.Sprintf(`command="echo This key permits tunnels only",permitopen="fakehost:1",permitlisten="%s:%d"`, bindAddr, port)

	tunnelId := fmt.Sprintf("boringproxy-%s-%d", domain, port)

	newAk := fmt.Sprintf("%s%s %s %s\n", akStr, options, pubKey, tunnelId)

	// Clear the file
	err = akFile.Truncate(0)
	if err != nil {
		return "", err
	}
	_, err = akFile.Seek(0, 0)
	if err != nil {
		return "", err
	}

	_, err = akFile.Write([]byte(newAk))
	if err != nil {
		return "", err
	}

	return privKey, nil
}

// Adapted from https://stackoverflow.com/a/34347463/943814
// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func MakeSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", err
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}

	pubKey := string(ssh.MarshalAuthorizedKey(pub))

	return pubKey, privKeyBuf.String(), nil
}
