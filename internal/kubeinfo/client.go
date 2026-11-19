// Package kubeinfo reads a small, read-only summary of a Kubernetes cluster
// (nodes, pods, deployments, Argo CD applications) over the plain HTTP API,
// so the dashboard needs no client-go dependency. It uses the in-cluster
// service account when running in a pod and a kubeconfig file otherwise.
package kubeinfo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zekihan/sdash/config"
)

const (
	defaultTimeout    = 8 * time.Second
	maxResponseBytes  = 32 << 20
	serviceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount"
)

var (
	ErrNoConfig    = errors.New("no kubernetes configuration found")
	ErrBadConfig   = errors.New("invalid kubernetes configuration")
	ErrUnavailable = errors.New("kubernetes API unavailable")
	ErrBadResponse = errors.New("invalid kubernetes API response")
)

// Client talks to one Kubernetes API server.
type Client struct {
	httpClient *http.Client
	base       string
	token      string
	tokenFile  string
	argocdNS   string
}

type transportConfig struct {
	server    string
	token     string
	tokenFile string
	caData    []byte
	certData  []byte
	keyData   []byte
	insecure  bool
}

// New builds a client from the config: explicit kubeconfig path, else the
// in-cluster service account, else ~/.kube/config. KUBECONFIG (first entry)
// is honored when no path is configured.
func New(cfg config.KubernetesConfig) (*Client, error) {
	return newClient(cfg, serviceAccountDir)
}

func newClient(cfg config.KubernetesConfig, accountDir string) (*Client, error) {
	transport, err := loadTransport(cfg.Kubeconfig, accountDir)
	if err != nil {
		return nil, err
	}

	tlsConfig := new(tls.Config)
	tlsConfig.MinVersion = tls.VersionTLS12
	tlsConfig.InsecureSkipVerify = transport.insecure

	if len(transport.caData) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(transport.caData) {
			return nil, fmt.Errorf("%w: certificate authority data did not parse", ErrBadConfig)
		}

		tlsConfig.RootCAs = pool
	}

	if len(transport.certData) > 0 && len(transport.keyData) > 0 {
		cert, certErr := tls.X509KeyPair(transport.certData, transport.keyData)
		if certErr != nil {
			return nil, fmt.Errorf("%w: client key pair: %w", ErrBadConfig, certErr)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	httpTransport := new(http.Transport)
	httpTransport.TLSClientConfig = tlsConfig
	httpTransport.Proxy = http.ProxyFromEnvironment

	httpClient := new(http.Client)
	httpClient.Timeout = defaultTimeout
	httpClient.Transport = httpTransport

	client := new(Client)
	client.httpClient = httpClient
	client.base = strings.TrimSuffix(transport.server, "/")
	client.token = transport.token
	client.tokenFile = transport.tokenFile
	client.argocdNS = cfg.ArgocdNamespace

	if client.argocdNS == "" {
		client.argocdNS = "argocd"
	}

	return client, nil
}

func loadTransport(kubeconfigPath, accountDir string) (*transportConfig, error) {
	if kubeconfigPath != "" {
		return loadKubeconfig(kubeconfigPath)
	}

	inCluster, err := loadInCluster(accountDir)
	if err == nil {
		return inCluster, nil
	}

	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		first, _, _ := strings.Cut(envPath, string(os.PathListSeparator))

		return loadKubeconfig(first)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, ErrNoConfig
	}

	return loadKubeconfig(filepath.Join(home, ".kube", "config"))
}

func loadInCluster(accountDir string) (*transportConfig, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")

	if host == "" || port == "" {
		return nil, ErrNoConfig
	}

	tokenFile := filepath.Join(accountDir, "token")

	_, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, ErrNoConfig
	}

	transport := new(transportConfig)
	transport.server = "https://" + net.JoinHostPort(host, port)
	transport.tokenFile = tokenFile

	caData, err := os.ReadFile(filepath.Join(accountDir, "ca.crt"))
	if err == nil {
		transport.caData = caData
	}

	return transport, nil
}

// kubeconfigFile mirrors the kubeconfig YAML schema; key names are dictated
// by that format.
//
//nolint:tagliatelle // kubeconfig uses kebab-case keys
type kubeconfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
			CertificateAuthority     string `yaml:"certificate-authority"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Contexts []struct {
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
		Name string `yaml:"name"`
	} `yaml:"contexts"`
	Users []struct {
		User struct {
			Exec                  any    `yaml:"exec"`
			Token                 string `yaml:"token"`
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientCertificate     string `yaml:"client-certificate"`
			ClientKeyData         string `yaml:"client-key-data"`
			ClientKey             string `yaml:"client-key"`
		} `yaml:"user"`
		Name string `yaml:"name"`
	} `yaml:"users"`
}

func loadKubeconfig(path string) (*transportConfig, error) {
	content, err := os.ReadFile(path) //nolint:gosec // operator-configured kubeconfig path
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrNoConfig, path, err)
	}

	parsed := new(kubeconfigFile)

	err = yaml.Unmarshal(content, parsed)
	if err != nil {
		return nil, fmt.Errorf("%w: parse %s: %w", ErrBadConfig, path, err)
	}

	return pickContext(parsed, filepath.Dir(path))
}

func pickContext(parsed *kubeconfigFile, baseDir string) (*transportConfig, error) {
	transport := new(transportConfig)

	clusterName := parsed.CurrentContext
	userName := ""

	for _, entry := range parsed.Contexts {
		if entry.Name == parsed.CurrentContext {
			clusterName = entry.Context.Cluster
			userName = entry.Context.User
		}
	}

	for _, entry := range parsed.Clusters {
		if entry.Name != clusterName && len(parsed.Clusters) != 1 {
			continue
		}

		transport.server = entry.Cluster.Server
		transport.insecure = entry.Cluster.InsecureSkipTLSVerify

		data, err := fileOrData(
			baseDir,
			entry.Cluster.CertificateAuthority,
			entry.Cluster.CertificateAuthorityData,
		)
		if err != nil {
			return nil, err
		}

		transport.caData = data

		break
	}

	if transport.server == "" {
		return nil, fmt.Errorf(
			"%w: no cluster server for context %q",
			ErrBadConfig,
			parsed.CurrentContext,
		)
	}

	for _, entry := range parsed.Users {
		if entry.Name != userName && len(parsed.Users) != 1 {
			continue
		}

		if entry.User.Exec != nil {
			return nil, fmt.Errorf("%w: exec credential plugins are not supported", ErrBadConfig)
		}

		transport.token = entry.User.Token

		certData, err := fileOrData(
			baseDir,
			entry.User.ClientCertificate,
			entry.User.ClientCertificateData,
		)
		if err != nil {
			return nil, err
		}

		keyData, err := fileOrData(baseDir, entry.User.ClientKey, entry.User.ClientKeyData)
		if err != nil {
			return nil, err
		}

		transport.certData = certData
		transport.keyData = keyData

		break
	}

	return transport, nil
}

func fileOrData(baseDir, file, data string) ([]byte, error) {
	if data != "" {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("%w: decode embedded data: %w", ErrBadConfig, err)
		}

		return decoded, nil
	}

	if file == "" {
		return nil, nil
	}

	path := file
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	content, err := os.ReadFile(path) //nolint:gosec // path from operator's kubeconfig
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrBadConfig, path, err)
	}

	return content, nil
}

func (client *Client) newRequest(
	ctx context.Context,
	method, apiPath string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, client.base+apiPath, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	req.Header.Set("Accept", "application/json")

	token := client.token
	if client.tokenFile != "" {
		// The kubelet replaces projected service account tokens as they rotate.
		content, readErr := os.ReadFile(client.tokenFile)
		if readErr != nil {
			return nil, fmt.Errorf("%w: read service account token: %w", ErrUnavailable, readErr)
		}

		token = strings.TrimSpace(string(content))
		if token == "" {
			return nil, fmt.Errorf("%w: empty service account token", ErrUnavailable)
		}
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req, nil
}

// getJSON performs an authenticated GET against the API server.
func (client *Client) getJSON(ctx context.Context, apiPath string, out any) error {
	req, err := client.newRequest(ctx, http.MethodGet, apiPath, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: GET %s returned %s", ErrUnavailable, apiPath, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	err = json.Unmarshal(body, out)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadResponse, err)
	}

	return nil
}
