package boringproxy

import (
	"flag"
	"fmt"
	"os"
	"strconv"
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

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

// Helper to read an environment variable into a bool or return default value
func getEnvAsBool(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}

	return defaultVal
}

func SetServerConfig(flags []string) *ServerConfig {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	adminDomain := flagSet.String("admin-domain", getEnv("BP_ADMIN_DOMAIN", ""), "Admin Domain")
	sshServerPort := flagSet.Int("ssh-server-port", getEnvAsInt("BP_SSH_SERVER_PORT", 22), "SSH Server Port")
	dbDir := flagSet.String("db-dir", getEnv("BP_DB_DIR", ""), "Database file directory")
	printLogin := flagSet.Bool("print-login", getEnvAsBool("BP_PRINT_LOGIN", false), "Prints admin login information")
	httpPort := flagSet.Int("http-port", getEnvAsInt("BP_HTTP_PORT", 80), "HTTP (insecure) port")
	httpsPort := flagSet.Int("https-port", getEnvAsInt("BP_HTTPS_PORT", 443), "HTTPS (secure) port")
	allowHttp := flagSet.Bool("allow-http", getEnvAsBool("BP_ALLOW_HTTP", false), "Allow unencrypted (HTTP) requests")
	publicIp := flagSet.String("public-ip", getEnv("BP_PUBLIC_IP", ""), "Public IP")
	behindProxy := flagSet.Bool("behind-proxy", getEnvAsBool("BP_BEHIND_PROXY", false), "Whether we're running behind another reverse proxy")
	certDir := flagSet.String("cert-dir", getEnv("BP_CERT_DIR", ""), "TLS cert directory")
	acmeEmail := flagSet.String("acme-email", getEnv("BP_ACME_EMAIL", ""), "Email for ACME (ie Let's Encrypt)")
	defaultCA := flagSet.String("ca", getEnv("BP_CA", "production"), "Default ACME CA")
	autoCerts := flagSet.Bool("autocert", getEnvAsBool("BP_AUTO_CERTS", false), "Enable/Disable auto certs")

	err := flagSet.Parse(flags)
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

func SetClientConfig(flags []string) *ClientConfig {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	server := flagSet.String("server", getEnv("BP_SERVER", ""), "boringproxy server")
	token := flagSet.String("token", getEnv("BP_TOKEN", ""), "Access token")
	name := flagSet.String("client-name", getEnv("BP_CLIENT_NAME", ""), "Client name")
	user := flagSet.String("user", getEnv("BP_USER", ""), "user")
	dnsServer := flagSet.String("dns-server", getEnv("BP_DNS_SERVER", ""), "Custom DNS server")
	behindProxy := flagSet.Bool("behind-proxy", getEnvAsBool("BP_BEHIND_PROXY", false), "Whether we're running behind another reverse proxy")
	certDir := flagSet.String("cert-dir", getEnv("BP_CERT_DIR", ""), "TLS cert directory")
	acmeEmail := flagSet.String("acme-email", getEnv("BP_ACME_EMAIL", ""), "Email for ACME (ie Let's Encrypt)")
	defaultCA := flagSet.String("ca", getEnv("BP_CA", "production"), "Default ACME CA")
	autoCerts := flagSet.Bool("autocert", getEnvAsBool("BP_AUTO_CERTS", false), "Enable/Disable auto certs")

	err := flagSet.Parse(flags)
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
