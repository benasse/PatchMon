package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

const userAgent = "patchmon-cli/0.1"

const (
	connectionModePTYAgent    = "PTY Agent"
	connectionModeAgentTunnel = "Agent Tunnel"
)

type config struct {
	Server      string `json:"server"`
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	Username    string `json:"username,omitempty"`
	Insecure    bool   `json:"insecure,omitempty"`
}
type loginResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at"`
	RequiresTFA bool   `json:"requiresTfa"`
	Username    string `json:"username"`
	User        struct {
		Username string `json:"username"`
	} `json:"user"`
}
type host struct {
	ID              string       `json:"id"`
	FriendlyName    string       `json:"friendly_name"`
	Hostname        *string      `json:"hostname"`
	IP              *string      `json:"ip"`
	OSType          string       `json:"os_type"`
	OSVersion       string       `json:"os_version"`
	Status          string       `json:"status"`
	EffectiveStatus string       `json:"effectiveStatus"`
	LastUpdate      string       `json:"last_update"`
	APIID           string       `json:"api_id"`
	Groups          []hostGroup  `json:"groups,omitempty"`
	AllowedAccounts []sshAccount `json:"allowed_accounts,omitempty"`
}
type hostGroup struct {
	Name string `json:"name"`
}
type sshAccount struct {
	LinuxUsername string `json:"linux_username"`
	AllowSudo     bool   `json:"allow_sudo"`
}
type apiClient struct {
	cfg  config
	http *http.Client
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "patchmon:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	cmd := newRootCommand()
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "patchmon",
		Short:         "PatchMon CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	var loginServer, loginUsername string
	var loginInsecure bool
	loginCmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"init"},
		Short:   "Log in to a PatchMon server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(loginServer, loginUsername, loginInsecure)
		},
	}
	loginCmd.Flags().StringVar(&loginServer, "server", "", "PatchMon server URL")
	loginCmd.Flags().StringVar(&loginUsername, "username", "", "PatchMon username or email")
	loginCmd.Flags().BoolVar(&loginInsecure, "insecure", false, "allow HTTP or skip TLS verification")

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out and remove the saved token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout()
		},
	}

	var hostsOutput string
	hostsCmd := &cobra.Command{
		Use:   "hosts",
		Short: "List PatchMon hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHosts(hostsOutput)
		},
	}
	hostsCmd.Flags().StringVar(&hostsOutput, "output", "table", "table or json")
	_ = hostsCmd.RegisterFlagCompletionFunc("output", fixedCompletions("table", "json"))
	hostsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List PatchMon hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHosts(hostsOutput)
		},
	}
	hostsListCmd.Flags().StringVar(&hostsOutput, "output", "table", "table or json")
	_ = hostsListCmd.RegisterFlagCompletionFunc("output", fixedCompletions("table", "json"))
	hostsCmd.AddCommand(hostsListCmd)

	sshCmd := &cobra.Command{
		Use:               "ssh user@host",
		Short:             "Open a recorded PTY Agent SSH session",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSSHTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSH(args[0])
		},
	}

	sshTunnelCmd := &cobra.Command{
		Use:               "ssh-tunnel host",
		Short:             "Open a raw Agent Tunnel SSH stream",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHostArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSHTunnel(args[0])
		},
	}

	root.AddCommand(loginCmd, logoutCmd, hostsCmd, sshCmd, sshTunnelCmd)
	root.AddCommand(completionCommand(root))
	root.SetHelpCommand(&cobra.Command{Use: "help [command]", Short: "Help about any command"})
	return root
}

func fixedCompletions(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if strings.HasPrefix(value, toComplete) {
				out = append(out, value)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
func configPath() (string, error) {
	d, err := os.UserConfigDir()
	return filepath.Join(d, "patchmon", "config.json"), err
}
func loadConfig() (config, error) {
	p, err := configPath()
	if err != nil {
		return config{}, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return config{}, errors.New("not logged in; run 'patchmon login --server URL'")
	}
	if err != nil {
		return config{}, err
	}
	var cfg config
	if json.Unmarshal(b, &cfg) != nil || cfg.Server == "" || cfg.AccessToken == "" {
		return config{}, errors.New("invalid configuration; run 'patchmon login' again")
	}
	return cfg, nil
}
func saveConfig(cfg config) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".config-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
func runLogout() error {
	p, err := configPath()
	if err == nil {
		err = os.Remove(p)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Println("Logged out")
	return nil
}

func completionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion bash|zsh|fish",
		Short:                 "Generate shell completion scripts",
		Args:                  cobra.ExactArgs(1),
		ValidArgsFunction:     fixedCompletions("bash", "zsh", "fish"),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			default:
				return fmt.Errorf("unsupported shell %q; expected bash, zsh, or fish", args[0])
			}
		},
	}
	return cmd
}

func completeHostArgs(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return hostCompletionValues(toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeSSHTargets(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return hostCompletionValues(toComplete), cobra.ShellCompDirectiveNoFileComp
}

func hostCompletionValues(prefix string) []string {
	cfg, err := loadConfig()
	if err != nil {
		return nil
	}
	hosts, err := newAPIClient(cfg).hosts(context.Background())
	if err != nil {
		return nil
	}
	userPrefix := ""
	hostPrefix := prefix
	if before, after, ok := strings.Cut(prefix, "@"); ok {
		userPrefix = before + "@"
		hostPrefix = after
	}
	seen := map[string]struct{}{}
	var out []string
	for _, host := range hosts {
		for _, candidate := range hostCompletionCandidates(host) {
			if !strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(hostPrefix)) {
				continue
			}
			value := userPrefix + candidate
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func hostCompletionCandidates(h host) []string {
	raw := []string{h.FriendlyName, value(h.Hostname), value(h.IP), h.ID}
	out := make([]string, 0, len(raw))
	for _, candidate := range raw {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == "-" || strings.ContainsAny(candidate, " \t\n\r") {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func runLogin(server, username string, insecure bool) error {
	if server == "" {
		if old, err := loadConfig(); err == nil {
			server = old.Server
		}
	}
	if server == "" {
		return errors.New("--server is required")
	}
	normalized, err := normalizeServer(server, insecure)
	if err != nil {
		return err
	}
	if username == "" {
		fmt.Print("Username or email: ")
		if _, err := fmt.Scanln(&username); err != nil {
			return errors.New("username is required")
		}
	}
	password, err := readSecret("Password: ")
	if err != nil {
		return err
	}
	client := newAPIClient(config{Server: normalized, Insecure: insecure})
	var out loginResponse
	err = client.post(context.Background(), "/api/v1/auth/login", map[string]string{"username": username, "password": password}, &out, false)
	password = ""
	if err != nil {
		return err
	}
	if out.RequiresTFA {
		code, err := readSecret("Authentication code: ")
		if err != nil {
			return err
		}
		err = client.post(context.Background(), "/api/v1/auth/verify-tfa", map[string]interface{}{"username": out.Username, "token": strings.TrimSpace(code), "remember_me": false}, &out, false)
		code = ""
		if err != nil {
			return err
		}
	}
	token := out.AccessToken
	if token == "" {
		token = out.Token
	}
	if token == "" {
		return errors.New("server did not return an access token")
	}
	name := out.User.Username
	if name == "" {
		name = username
	}
	if err := saveConfig(config{Server: normalized, AccessToken: token, ExpiresAt: out.ExpiresAt, Username: name, Insecure: insecure}); err != nil {
		return err
	}
	fmt.Printf("Logged in to %s as %s\n", normalized, name)
	return nil
}
func normalizeServer(raw string, insecure bool) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("invalid server URL")
	}
	if u.Scheme != "https" && !(insecure && u.Scheme == "http") {
		return "", errors.New("server must use HTTPS (or --insecure for HTTP development instances)")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("server URL must not contain a query or fragment")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}
func newAPIClient(cfg config) *apiClient {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.Insecure {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return &apiClient{cfg: cfg, http: &http.Client{Transport: t, Timeout: 30 * time.Second}}
}
func (c *apiClient) request(ctx context.Context, method, path string, body, out interface{}, auth bool) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.Server+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("contact server: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&e)
		msg := e.Error
		if msg == "" {
			msg = e.Message
		}
		if msg == "" {
			msg = resp.Status
		}
		if resp.StatusCode == http.StatusUnauthorized {
			msg += "; run 'patchmon login' again"
		}
		return errors.New(msg)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
func (c *apiClient) post(ctx context.Context, path string, body, out interface{}, auth bool) error {
	return c.request(ctx, http.MethodPost, path, body, out, auth)
}
func (c *apiClient) hosts(ctx context.Context) ([]host, error) {
	var out []host
	err := c.request(ctx, http.MethodGet, "/api/v1/dashboard/hosts", nil, &out, true)
	return out, err
}
func runHosts(output string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client := newAPIClient(cfg)
	hosts, err := client.hosts(context.Background())
	if err != nil {
		return err
	}
	sort.Slice(hosts, func(i, j int) bool { return displayName(hosts[i]) < displayName(hosts[j]) })
	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hosts)
	}
	if output != "table" {
		return errors.New("--output must be table or json")
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tHOSTNAME\tIP\tOS\tSTATUS\tGROUPS\tLAST UPDATE")
	for _, h := range hosts {
		status := h.EffectiveStatus
		if status == "" {
			status = h.Status
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s %s\t%s\t%s\t%s\n", displayName(h), value(h.Hostname), value(h.IP), h.OSType, h.OSVersion, status, groupNames(h.Groups), h.LastUpdate)
	}
	return w.Flush()
}
func groupNames(groups []hostGroup) string {
	if len(groups) == 0 {
		return "-"
	}
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	return strings.Join(names, ",")
}
func displayName(h host) string {
	if h.FriendlyName != "" {
		return h.FriendlyName
	}
	if h.Hostname != nil && *h.Hostname != "" {
		return *h.Hostname
	}
	return h.ID
}
func value(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}
func resolveHost(hosts []host, query string) (host, error) {
	var found []host
	for _, h := range hosts {
		if h.ID == query || h.APIID == query || strings.EqualFold(h.FriendlyName, query) || (h.Hostname != nil && strings.EqualFold(*h.Hostname, query)) {
			found = append(found, h)
		}
	}
	if len(found) == 0 {
		return host{}, fmt.Errorf("host %q not found", query)
	}
	if len(found) > 1 {
		return host{}, fmt.Errorf("host %q is ambiguous; use its ID", query)
	}
	return found[0], nil
}
func runSSH(targetArg string) error {
	user, target, ok := strings.Cut(targetArg, "@")
	if !ok || user == "" || target == "" {
		return errors.New("a Linux account is required: patchmon ssh user@host")
	}
	if user == "root" {
		return errors.New("root login is disabled by default")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client := newAPIClient(cfg)
	hosts, err := client.hosts(context.Background())
	if err != nil {
		return err
	}
	host, err := resolveHost(hosts, target)
	if err != nil {
		return err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ephemeral SSH key: %w", err)
	}
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("encode ephemeral SSH key: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("encode ephemeral private key: %w", err)
	}
	for i := range privateKey {
		privateKey[i] = 0
	}
	var certificate struct {
		Certificate string `json:"certificate"`
		CAPublicKey string `json:"caPublicKey"`
		BastionHost string `json:"bastionHost"`
		BastionPort string `json:"bastionPort"`
		ExpiresAt   string `json:"expiresAt"`
		Recorded    bool   `json:"recorded"`
	}
	request := map[string]string{"hostId": host.ID, "linuxUsername": user, "publicKey": string(ssh.MarshalAuthorizedKey(sshPublicKey))}
	if err := client.post(context.Background(), "/api/v1/auth/ssh-certificate", request, &certificate, true); err != nil {
		return err
	}
	if certificate.Certificate == "" || certificate.BastionHost == "" || certificate.BastionPort == "" {
		return errors.New("server returned an incomplete SSH certificate")
	}
	tempDir, err := os.MkdirTemp("", "patchmon-ssh-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	keyPath := filepath.Join(tempDir, "id_ed25519")
	certPath := filepath.Join(tempDir, "id_ed25519-cert.pub")
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	for i := range privateDER {
		privateDER[i] = 0
	}
	if err := os.WriteFile(keyPath, privatePEM, 0600); err != nil {
		return err
	}
	for i := range privatePEM {
		privatePEM[i] = 0
	}
	if err := os.WriteFile(certPath, []byte(certificate.Certificate), 0600); err != nil {
		return err
	}
	if certificate.Recorded {
		fmt.Fprintf(os.Stderr, "PatchMon: connection mode %s; this interactive session is recorded (certificate expires %s).\n", connectionModePTYAgent, certificate.ExpiresAt)
	}
	command := exec.Command("ssh", "-p", certificate.BastionPort, "-i", keyPath, "-o", "CertificateFile="+certPath, "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=accept-new", user+"@"+certificate.BastionHost)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("OpenSSH failed: %w", err)
	}
	return nil
}

func runSSHTunnel(target string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client := newAPIClient(cfg)
	hosts, err := client.hosts(context.Background())
	if err != nil {
		return err
	}
	host, err := resolveHost(hosts, target)
	if err != nil {
		return err
	}
	var ticket struct {
		Ticket string `json:"ticket"`
	}
	if err := client.post(context.Background(), "/api/v1/auth/ssh-ticket", map[string]string{"hostId": host.ID}, &ticket, true); err != nil {
		return err
	}
	wsURL, err := websocketEndpointURL(cfg.Server, "ssh-tunnel", host.ID, ticket.Ticket)
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 20 * time.Second}
	if cfg.Insecure {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	conn, response, err := dialer.Dial(wsURL, http.Header{"User-Agent": []string{userAgent}})
	if err != nil {
		if response != nil {
			return fmt.Errorf("open SSH tunnel: %s", response.Status)
		}
		return fmt.Errorf("open SSH tunnel: %w", err)
	}
	defer conn.Close()
	go func() {
		buffer := make([]byte, 32*1024)
		for {
			n, readErr := os.Stdin.Read(buffer)
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}
		if messageType != websocket.BinaryMessage {
			continue
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
	}
}
func websocketURL(server, hostID, ticket string) (string, error) {
	return websocketEndpointURL(server, "ssh-terminal", hostID, ticket)
}

func websocketEndpointURL(server, endpoint, hostID, ticket string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/" + endpoint + "/" + hostID
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func terminalName() string {
	if s := os.Getenv("TERM"); s != "" {
		return s
	}
	return "xterm-256color"
}
func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}
