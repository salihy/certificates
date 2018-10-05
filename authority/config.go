package authority

import (
	"encoding/json"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/smallstep/cli/crypto/tlsutil"
	"github.com/smallstep/cli/crypto/x509util"
	jose "gopkg.in/square/go-jose.v2"
)

// DefaultTLSOptions represents the default TLS version as well as the cipher
// suites used in the TLS certificates.
var DefaultTLSOptions = tlsutil.TLSOptions{
	CipherSuites: x509util.CipherSuites{
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	},
	MinVersion:    1.2,
	MaxVersion:    1.2,
	Renegotiation: false,
}

const (
	// minCertDuration is the minimum validity of an end-entity (not root or intermediate) certificate.
	minCertDuration = 5 * time.Minute
	// maxCertDuration is the maximum validity of an end-entity (not root or intermediate) certificate.
	maxCertDuration = 24 * time.Hour
)

type duration struct {
	time.Duration
}

// UnmarshalJSON parses a duration string and sets it to the duration.
//
// A duration string is a possibly signed sequence of decimal numbers, each with
// optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m".
// Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
func (d *duration) UnmarshalJSON(data []byte) (err error) {
	var s string
	if err = json.Unmarshal(data, &s); err != nil {
		return errors.Wrapf(err, "error unmarshalling %s", data)
	}
	if d.Duration, err = time.ParseDuration(s); err != nil {
		return errors.Wrapf(err, "error parsing %s as duration", s)
	}
	return
}

// Provisioner - authorized entity that can sign tokens necessary for signature requests.
type Provisioner struct {
	Issuer       string           `json:"issuer,omitempty"`
	Type         string           `json:"type,omitempty"`
	Key          *jose.JSONWebKey `json:"key,omitempty"`
	EncryptedKey string           `json:"encryptedKey,omitempty"`
}

// Config represents the CA configuration and it's mapped to a JSON object.
type Config struct {
	Root             string              `json:"root"`
	IntermediateCert string              `json:"crt"`
	IntermediateKey  string              `json:"key"`
	Address          string              `json:"address"`
	DNSNames         []string            `json:"dnsNames"`
	Logger           json.RawMessage     `json:"logger,omitempty"`
	Monitoring       json.RawMessage     `json:"monitoring,omitempty"`
	AuthorityConfig  *AuthConfig         `json:"authority,omitempty"`
	TLS              *tlsutil.TLSOptions `json:"tls,omitempty"`
	Password         string              `json:"password,omitempty"`
}

// AuthConfig represents the configuration options for the authority.
type AuthConfig struct {
	Provisioners    []*Provisioner   `json:"provisioners,omitempty"`
	Template        *x509util.ASN1DN `json:"template,omitempty"`
	MinCertDuration *duration        `json:"minCertDuration,omitempty"`
	MaxCertDuration *duration        `json:"maxCertDuration,omitempty"`
}

// Validate validates the authority configuration.
func (c *AuthConfig) Validate() error {
	switch {
	case c == nil:
		return errors.New("authority cannot be undefined")
	case len(c.Provisioners) == 0:
		return errors.New("authority.provisioners cannot be empty")
	default:
		if c.Template == nil {
			c.Template = &x509util.ASN1DN{}
		}
		return nil
	}
}

// LoadConfiguration parses the given filename in JSON format and returns the
// configuration struct.
func LoadConfiguration(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %s", filename)
	}
	defer f.Close()

	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, errors.Wrapf(err, "error parsing %s", filename)
	}

	return &c, nil
}

// Save saves the configuration to the given filename.
func (c *Config) Save(filename string) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrapf(err, "error opening %s", filename)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")
	return errors.Wrapf(enc.Encode(c), "error writing %s", filename)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	switch {
	case c.Address == "":
		return errors.New("address cannot be empty")

	case c.Root == "":
		return errors.New("root cannot be empty")

	case c.IntermediateCert == "":
		return errors.New("crt cannot be empty")

	case c.IntermediateKey == "":
		return errors.New("key cannot be empty")

	case len(c.DNSNames) == 0:
		return errors.New("dnsNames cannot be empty")
	}

	if c.TLS == nil {
		c.TLS = &DefaultTLSOptions
	} else {
		if len(c.TLS.CipherSuites) == 0 {
			c.TLS.CipherSuites = DefaultTLSOptions.CipherSuites
		}
		if c.TLS.MaxVersion == 0 {
			c.TLS.MaxVersion = DefaultTLSOptions.MaxVersion
		}
		if c.TLS.MinVersion == 0 {
			c.TLS.MinVersion = c.TLS.MaxVersion
		}
		if c.TLS.MinVersion > c.TLS.MaxVersion {
			return errors.New("tls minVersion cannot exceed tls maxVersion")
		}
		c.TLS.Renegotiation = c.TLS.Renegotiation || DefaultTLSOptions.Renegotiation
	}

	if err := c.AuthorityConfig.Validate(); err != nil {
		return err
	}

	return nil
}