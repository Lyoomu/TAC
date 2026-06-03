package security

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"strings"
)

func NewTLSConfig(trustedFingerprint string) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS13,
	}

	config.InsecureSkipVerify = true

	if trustedFingerprint == "" || trustedFingerprint == "insecure" {
		config.VerifyConnection = func(cs tls.ConnectionState) error {
			return nil
		}
	} else {

		config.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("no peer certificate")
			}
			actual := GetFingerprint(cs)
			if actual != trustedFingerprint {
				return fmt.Errorf(
					"certificate fingerprint mismatch:\n  expected: %s\n  actual:   %s",
					trustedFingerprint, actual,
				)
			}
			return nil
		}
	}

	return config, nil
}

func GetFingerprint(state tls.ConnectionState) string {
	if len(state.PeerCertificates) == 0 {
		return ""
	}
	cert := state.PeerCertificates[0]
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

func FingerprintFromString(s string) string {

	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ToLower(s)
	return s
}
