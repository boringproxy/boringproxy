package boringproxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"os/user"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
	"golang.org/x/crypto/ssh"
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

/* X509 can not marshall a ed25519 key for some reason, even in 2022.
The following code is from mikesmitty/edkey */

/* Writes ed25519 private keys into the new OpenSSH private key format.
I have no idea why this isn't implemented anywhere yet, you can do seemingly
everything except write it to disk in the OpenSSH private key format. */
func MarshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// Add our key header (followed by a null byte)
	magic := append([]byte("openssh-key-v1"), 0)

	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Fill out the private key fields
	pk1 := struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}{}

	// Set our check ints
	ci := mrand.Uint32()
	pk1.Check1 = ci
	pk1.Check2 = ci

	// Set our key type
	pk1.Keytype = ssh.KeyAlgoED25519

	// Add the pubkey to the optionally-encrypted block
	pk, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		//fmt.Fprintln(os.Stderr, "ed25519.PublicKey type assertion failed on an ed25519 public key. This should never ever happen.")
		return nil
	}
	pubKey := []byte(pk)
	pk1.Pub = pubKey

	// Add our private key
	pk1.Priv = []byte(key)

	// Might be useful to put something in here at some point
	pk1.Comment = ""

	// Add some padding to match the encryption block size within PrivKeyBlock (without Pad field)
	// 8 doesn't match the documentation, but that's what ssh-keygen uses for unencrypted keys. *shrug*
	bs := 8
	blockLen := len(ssh.Marshal(pk1))
	padLen := (bs - (blockLen % bs)) % bs
	pk1.Pad = make([]byte, padLen)

	// Padding is a sequence of bytes like: 1, 2, 3...
	for i := 0; i < padLen; i++ {
		pk1.Pad[i] = byte(i + 1)
	}

	// Generate the pubkey prefix "\0\0\0\nssh-ed25519\0\0\0 "
	prefix := []byte{0x0, 0x0, 0x0, 0x0b}
	prefix = append(prefix, []byte(ssh.KeyAlgoED25519)...)
	prefix = append(prefix, []byte{0x0, 0x0, 0x0, 0x20}...)

	// Only going to support unencrypted keys for now
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""
	w.NumKeys = 1
	w.PubKey = append(prefix, pubKey...)
	w.PrivKeyBlock = ssh.Marshal(pk1)

	magic = append(magic, ssh.Marshal(w)...)

	return magic
}

/* end */

// Generate SSH key pair using ed25519
// This will invalidate any setup using 0.9.1-beta or below!
func MakeSSHKeyPair() (string, string, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: MarshalED25519PrivateKey(privateKey),
	}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", "", err
	}

	pubKey := string(ssh.MarshalAuthorizedKey(pub))

	return pubKey, privKeyBuf.String(), nil
}
