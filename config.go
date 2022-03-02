package boringproxy

import (
	"flag"
	"fmt"
	"os"
)

type myCertConfig struct {
	certDir   string
	acmeEmail string
	defaultCA string
	autoCerts bool
}

type ServerConfig struct {
	adminDomain   string
	sshServerPort int
	dbDir         string
	printLogin    bool
	httpPort      int
	httpsPort     int
	allowHttp     bool
	publicIp      string
	behindProxy   bool
	myCertConfig  myCertConfig
}

type ClientConfig struct {
	ServerAddr     string `json:"serverAddr,omitempty"`
	Token          string `json:"token,omitempty"`
	ClientName     string `json:"clientName,omitempty"`
	User           string `json:"user,omitempty"`
	CertDir        string `json:"certDir,omitempty"`
	AcmeEmail      string `json:"acmeEmail,omitempty"`
	AcmeUseStaging bool   `json:"acmeUseStaging,omitempty"`
	DnsServer      string `json:"dnsServer,omitempty"`
	BehindProxy    bool   `json:"behindProxy,omitempty"`
	myCertConfig   myCertConfig
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func SetServerConfig() *ServerConfig {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	adminDomain := flagSet.String("admin-domain", "BP_ADMIN_DOMAIN", "Admin Domain")
	sshServerPort := flagSet.Int("ssh-server-port", 22, "SSH Server Port")
	dbDir := flagSet.String("db-dir", "", "Database file directory")
	printLogin := flagSet.Bool("print-login", false, "Prints admin login information")
	httpPort := flagSet.Int("http-port", 80, "HTTP (insecure) port")
	httpsPort := flagSet.Int("https-port", 443, "HTTPS (secure) port")
	allowHttp := flagSet.Bool("allow-http", false, "Allow unencrypted (HTTP) requests")
	publicIp := flagSet.String("public-ip", "", "Public IP")
	behindProxy := flagSet.Bool("behind-proxy", false, "Whether we're running behind another reverse proxy")
	certDir := flagSet.String("cert-dir", "", "TLS cert directory")
	acmeEmail := flagSet.String("acme-email", "", "Email for ACME (ie Let's Encrypt)")
	defaultCA := flagSet.String("ca", "production", "Default ACME CA")
	autoCerts := flagSet.Bool("autocert", true, "Enable/Disable auto certs")

	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parsing flags: %s\n", os.Args[0], err)
	}

	var myCertConfig = &myCertConfig{
		certDir:   *certDir,
		acmeEmail: *acmeEmail,
		defaultCA: *defaultCA,
		autoCerts: *autoCerts,
	}

	var config = &ServerConfig{
		adminDomain:   *adminDomain,
		sshServerPort: *sshServerPort,
		dbDir:         *dbDir,
		printLogin:    *printLogin,
		httpPort:      *httpPort,
		httpsPort:     *httpsPort,
		allowHttp:     *allowHttp,
		publicIp:      *publicIp,
		behindProxy:   *behindProxy,
		myCertConfig:  *myCertConfig,
	}

	return config
}

func SetClientConfig() *ClientConfig {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", "", "boringproxy server")
	token := flagSet.String("token", "", "Access token")
	name := flagSet.String("client-name", "", "Client name")
	user := flagSet.String("user", "", "user")
	dnsServer := flagSet.String("dns-server", "", "Custom DNS server")
	behindProxy := flagSet.Bool("behind-proxy", false, "Whether we're running behind another reverse proxy")
	certDir := flagSet.String("cert-dir", "", "TLS cert directory")
	acmeEmail := flagSet.String("acme-email", "", "Email for ACME (ie Let's Encrypt)")
	defaultCA := flagSet.String("ca", "production", "Default ACME CA")
	autoCerts := flagSet.Bool("autocert", true, "Enable/Disable auto certs")

	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: parsing flags: %s\n", os.Args[0], err)
	}

	if *server == "" {
		fail("-server is required")
	}

	if *token == "" {
		fail("-token is required")
	}

	var myCertConfig = &myCertConfig{
		certDir:   *certDir,
		acmeEmail: *acmeEmail,
		defaultCA: *defaultCA,
		autoCerts: *autoCerts,
	}

	var config = &ClientConfig{
		ServerAddr:   *server,
		Token:        *token,
		ClientName:   *name,
		User:         *user,
		DnsServer:    *dnsServer,
		BehindProxy:  *behindProxy,
		myCertConfig: *myCertConfig,
	}

	return config
}
