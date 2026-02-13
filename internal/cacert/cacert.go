// Package cacert collects CA certificates from the hosts used across kustomize overlays
// and bundles them for Argo CD. It performs a validating TLS handshake (no InsecureSkipVerify)
// to capture the peer certificate chain only from hosts the system already trusts.
package cacert

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cjeanner/kustomap/internal/types"
	"github.com/cjeanner/kustomap/internal/validation"
)

// DefaultTTL is how long a collected CA bundle and per-host cache entries are considered valid.
const DefaultTTL = 24 * time.Hour

// cacheEntry holds a per-host PEM with its expiry time.
type cacheEntry struct {
	pem       string
	expiresAt time.Time
}

// Collector collects CA certificates from unique hosts in a graph,
// uses a per-host cache with TTL for reuse across analyses, and attaches
// the resulting PEM bundle to the graph.
type Collector struct {
	// ttl is how long collected certs are cached per-host and how long
	// the graph's bundle is considered valid.
	ttl time.Duration
	// mu protects the per-host cache.
	mu    sync.RWMutex
	cache map[string]cacheEntry
}

// NewCollector creates a Collector with the given TTL for cache and bundle expiry.
// Pass DefaultTTL or a custom duration (e.g. 12*time.Hour).
func NewCollector(ttl time.Duration) *Collector {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Collector{
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
	}
}

// CollectAndAttach gathers unique hosts from the graph, collects their CA certs via
// a validating TLS handshake, builds a deduplicated PEM bundle, and sets it on the graph.
// Per-host PEMs are cached with TTL for reuse across graphs.
// Safe for concurrent use.
func (c *Collector) CollectAndAttach(graph *types.Graph) {
	hosts := c.uniqueHostsFromGraph(graph)
	if len(hosts) == 0 {
		log.Printf("CA bundle: no HTTPS hosts in graph")
		return
	}

	// Collect certs from each host (cache hit or TLS dial); deduplicate by fingerprint
	// since the same CA may sign multiple hosts.
	seenFingerprint := make(map[string]bool)
	var uniqueCerts []*x509.Certificate

	for _, host := range hosts {
		if err := validation.ValidateHost(host); err != nil {
			log.Printf("CA bundle: skip host %q (validation: %v)", host, err)
			continue
		}

		pemBlock, err := c.getPEMForHost(host)
		if err != nil {
			log.Printf("CA bundle: failed to get certs for %q: %v", host, err)
			continue
		}

		// Parse PEM and add only certs we haven't seen (by fingerprint).
		certs := parsePEMCerts(pemBlock)
		for _, cert := range certs {
			fp := certFingerprint(cert)
			if seenFingerprint[fp] {
				continue
			}
			seenFingerprint[fp] = true
			uniqueCerts = append(uniqueCerts, cert)
		}
	}

	if len(uniqueCerts) == 0 {
		log.Printf("CA bundle: no certs collected")
		return
	}

	var buf bytes.Buffer
	for _, cert := range uniqueCerts {
		_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	}

	graph.CABundle = buf.String()
	graph.CABundleExpires = time.Now().Add(c.ttl).Format(time.RFC3339)
	log.Printf("CA bundle: collected %d unique cert(s) from %d host(s)", len(uniqueCerts), len(hosts))
}

// uniqueHostsFromGraph extracts unique TLS hosts from the graph's BaseURLs.
// Resolves GitHub.com -> api.github.com (where the API and TLS connection go).
// Returns a sorted, deduplicated list (like sort -u).
func (c *Collector) uniqueHostsFromGraph(graph *types.Graph) []string {
	if graph == nil || graph.BaseURLs == nil {
		return nil
	}

	seen := make(map[string]bool)
	for _, baseURL := range graph.BaseURLs {
		if baseURL == "" {
			continue
		}
		host, err := resolveTLSHost(baseURL)
		if err != nil {
			log.Printf("CA bundle: invalid base URL %q: %v", baseURL, err)
			continue
		}
		seen[host] = true
	}

	var hosts []string
	for h := range seen {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return hosts
}

// resolveTLSHost maps a repo base URL to the host we actually connect to for TLS.
// GitHub.com uses api.github.com; GitHub Enterprise and GitLab use the URL host.
func resolveTLSHost(baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return "", nil
	}
	// GitHub.com (and default) uses the API subdomain; Enterprise uses same host.
	if host == "github.com" {
		return "api.github.com", nil
	}
	return host, nil
}

// getPEMForHost returns the PEM-encoded CA chain for the host, from cache or via TLS dial.
func (c *Collector) getPEMForHost(host string) (string, error) {
	c.mu.RLock()
	if ent, ok := c.cache[host]; ok && time.Now().Before(ent.expiresAt) {
		c.mu.RUnlock()
		return ent.pem, nil
	}
	c.mu.RUnlock()

	pemBlock, err := fetchCertsViaTLS(host)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.cache[host] = cacheEntry{pem: pemBlock, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return pemBlock, nil
}

// fetchCertsViaTLS performs a validating TLS handshake to the host and returns
// the peer certificate chain as PEM. No InsecureSkipVerify—only succeeds for
// hosts the system already trusts.
func fetchCertsViaTLS(host string) (string, error) {
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{})
	if err != nil {
		return "", err
	}
	defer conn.Close()

	state := conn.ConnectionState()
	// Use VerifiedChains when available (validation succeeded); otherwise PeerCertificates.
	// For a successful handshake, both are typically present.
	chains := state.VerifiedChains
	if len(chains) == 0 {
		chains = [][]*x509.Certificate{state.PeerCertificates}
	}

	var buf bytes.Buffer
	for _, chain := range chains {
		// Encode full chain (leaf + intermediates + root); Argo CD accepts this.
		// Leaf is redundant for verification but harmless.
		for _, cert := range chain {
			_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		}
	}

	return buf.String(), nil
}

// parsePEMCerts parses PEM blocks and returns the certificates.
func parsePEMCerts(pemData string) []*x509.Certificate {
	var certs []*x509.Certificate
	for block, rest := pem.Decode([]byte(pemData)); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		certs = append(certs, cert)
	}
	return certs
}

// certFingerprint returns a SHA256 fingerprint for deduplication.
func certFingerprint(cert *x509.Certificate) string {
	h := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(h[:])
}
