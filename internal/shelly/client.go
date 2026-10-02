package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/icholy/digest"
)

var ErrAuthentication = errors.New("authentication required or shared device password rejected")
var ErrIdentity = errors.New("the device at this address has a different identity")
var macPattern = regexp.MustCompile(`^[0-9A-F]{12}$`)

type Identity struct {
	ID      string `json:"id"`
	MAC     string `json:"mac"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	Gen     int    `json:"gen"`
	Version string `json:"ver"`
	Auth    bool   `json:"auth_en"`
}

type Reading struct {
	Identity        Identity
	Address         string
	Status          map[string]any
	Config          map[string]any
	ConfigAttempted bool
	Problem         string
}

type Client struct{ public, authenticated *http.Client }

func NewClient(password string) *Client {
	transport := &http.Transport{Proxy: nil, DialContext: dialLocal, ResponseHeaderTimeout: 3 * time.Second, TLSHandshakeTimeout: 3 * time.Second, MaxIdleConnsPerHost: 2}
	noRedirect := func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{
		public:        &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: noRedirect},
		authenticated: &http.Client{Transport: &digest.Transport{Username: "admin", Password: password, Transport: transport, NoReuse: true}, Timeout: 5 * time.Second, CheckRedirect: noRedirect},
	}
}

// Resolve and check the actual dial destination, including after DNS changes.
func dialLocal(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !ip.IP.IsPrivate() && !ip.IP.IsLoopback() && !ip.IP.IsLinkLocalUnicast() {
			return nil, errors.New("device addresses must resolve to the local network")
		}
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	if err == nil {
		err = errors.New("device address did not resolve")
	}
	return nil, err
}

func NormalizeAddress(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("enter a device hostname or HTTP/HTTPS address without a path or credentials")
	}
	u.Path = ""
	return u.String(), nil
}

func NormalizeMAC(value string) string {
	return strings.ToUpper(strings.NewReplacer(":", "", "-", "").Replace(value))
}

func (c *Client) Read(ctx context.Context, address, expectedMAC string, readConfig bool) (Reading, error) {
	address, err := NormalizeAddress(address)
	if err != nil {
		return Reading{}, err
	}
	reading := Reading{Address: address}
	if err := c.get(ctx, c.public, address+"/shelly", &reading.Identity); err != nil {
		return reading, err
	}
	reading.Identity.MAC = NormalizeMAC(reading.Identity.MAC)
	if reading.Identity.ID == "" || !macPattern.MatchString(reading.Identity.MAC) || reading.Identity.Gen < 2 {
		return reading, errors.New("no supported Gen2+ Shelly identity returned")
	}
	if expectedMAC != "" && reading.Identity.MAC != NormalizeMAC(expectedMAC) {
		return reading, ErrIdentity
	}
	if err := c.get(ctx, c.authenticated, address+"/rpc/Shelly.GetStatus", &reading.Status); err != nil {
		reading.Status = nil
		reading.Problem = describe(err)
	}
	if readConfig {
		reading.ConfigAttempted = true
		if err := c.get(ctx, c.authenticated, address+"/rpc/Shelly.GetConfig", &reading.Config); err != nil {
			reading.Config = nil
			reading.Problem = describe(err)
		}
		if reading.Config != nil {
			Redact(reading.Config)
		}
	}
	return reading, nil
}

// Discovery and refresh use the same bounded retries and identity validation.
func (c *Client) Discover(ctx context.Context, address string) (Reading, error) {
	return c.Read(ctx, address, "", true)
}

func DiscoveryError(err error) string {
	var networkError net.Error
	if errors.As(err, &networkError) {
		if networkError.Timeout() {
			return "Device connection timed out."
		}
		return "Device connection failed."
	}
	if errors.Is(err, ErrAuthentication) {
		return "Device authentication failed."
	}
	return "No supported device found at this address."
}

func (c *Client) get(ctx context.Context, client *http.Client, address string, dst any) error {
	err := c.getOnce(ctx, client, address, dst)
	var networkError net.Error
	var responseError *httpError
	transient := errors.As(err, &networkError) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	if errors.As(err, &responseError) {
		switch responseError.code {
		case 408, 429, 500, 502, 503, 504:
			transient = true
		}
	}
	if err == nil || ctx.Err() != nil || !transient {
		return err
	}
	timer := time.NewTimer(350 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return c.getOnce(ctx, client, address, dst)
	}
}

type httpError struct{ code int }

func (e *httpError) Error() string { return fmt.Sprintf("device returned HTTP %d", e.code) }

func (c *Client) getOnce(ctx context.Context, client *http.Client, address string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return ErrAuthentication
	}
	if response.StatusCode != 200 {
		return &httpError{code: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1048577))
	if err != nil {
		return err
	}
	if len(body) > 1048576 {
		return errors.New("device response too large")
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return errors.New("device returned an invalid JSON response")
	}
	return nil
}

func describe(err error) string {
	if errors.Is(err, ErrAuthentication) {
		return ErrAuthentication.Error()
	}
	return "Could not read device status or configuration. Check connectivity and device access."
}

// Targets bounds subnet discovery to a single private IPv4 subnet of at most /24.
func Targets(value string) ([]string, error) {
	if !strings.Contains(value, "/") || strings.Contains(value, "://") {
		address, err := NormalizeAddress(value)
		return []string{address}, err
	}
	prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() < 24 {
		return nil, errors.New("use a private IPv4 range no larger than /24, such as 192.168.1.0/24")
	}
	prefix = prefix.Masked()
	var addresses []string
	for ip := prefix.Addr(); ip.IsValid() && prefix.Contains(ip); ip = ip.Next() {
		addresses = append(addresses, "http://"+ip.String())
	}
	if prefix.Bits() <= 30 {
		addresses = addresses[1 : len(addresses)-1]
	}
	return addresses, nil
}

func Redact(value map[string]any) {
	for key, entry := range value {
		switch strings.ToLower(key) {
		case "pass", "password", "token", "ha1", "auth_key", "api_key", "private_key", "client_key":
			value[key] = "[redacted]"
			continue
		}
		switch nested := entry.(type) {
		case map[string]any:
			Redact(nested)
		case []any:
			for _, item := range nested {
				if object, ok := item.(map[string]any); ok {
					Redact(object)
				}
			}
		case string:
			if u, err := url.Parse(nested); err == nil && u.User != nil {
				u.User = nil
				value[key] = u.String()
			}
		}
	}
}
