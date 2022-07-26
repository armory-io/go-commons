package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/armory-io/lib-go-armory-cloud-commons/secrets"
	"io/ioutil"
	"os"
)

type (
	ClientTLSSettings struct {
		// Override server name to check on certificate
		ServerName string `yaml:"serverName,omitempty" json:"serverName,omitempty"`
		// Don't check server name
		InsecureSkipVerify bool `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
		// Key file if the cert file doesn't provide it
		ClientKeyFile string `yaml:"clientKeyFile,omitempty" json:"clientKeyFile,omitempty"`
		// Key password if the key is encrypted
		ClientKeyPassword string `yaml:"clientKeyFilePassword,omitempty" json:"clientKeyFilePassword,omitempty"`
		// If not provided, it will default to the system cacerts
		CAcertFile string `yaml:"cacertFile,omitempty" json:"cacertFile,omitempty"`
		// Client certificate file - useful for mTLS
		ClientCertFile string `yaml:"clientCertFile,omitempty" json:"clientCertFile,omitempty"`
	}
)

func GetTLSConfig(s *ClientTLSSettings) (*tls.Config, error) {
	cfg := tls.Config{}
	if s != nil {
		cfg.InsecureSkipVerify = s.InsecureSkipVerify
		if s.ServerName != "" {
			cfg.ServerName = s.ServerName
		}
		if err := addCaCert(&cfg, s); err != nil {
			return nil, err
		}
		if err := addClientOptions(&cfg, s); err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

func addClientOptions(cfg *tls.Config, s *ClientTLSSettings) error {
	if s.ClientCertFile == "" {
		return nil
	}

	cert, err := GetX509KeyPair(s.ClientCertFile, s.ClientKeyFile, s.ClientKeyPassword)
	if err != nil {
		return fmt.Errorf("error with certificate file %s: %w", s.ClientCertFile, err)
	}
	cfg.Certificates = []tls.Certificate{cert}
	cfg.PreferServerCipherSuites = true
	cfg.MinVersion = tls.VersionTLS12
	return nil
}

func addCaCert(cfg *tls.Config, s *ClientTLSSettings) error {
	// With mTLS, we'll parse our PEM to discover CAs with which to validate client certificates
	caFile := s.CAcertFile
	if caFile == "" {
		return nil
	}

	if err := CheckFileExists(caFile); err != nil {
		return fmt.Errorf("error with certificate authority file %s: %w", caFile, err)
	}

	// Create a CA certificate pool and add our server certificate
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	cfg.RootCAs = caCertPool
	return nil
}

func CheckFileExists(filename string) error {
	if secrets.IsEncryptedSecret(filename) {
		d, err := secrets.NewDecrypter(context.TODO(), filename)
		if err != nil {
			return err
		}
		if !d.IsFile() {
			return errors.New("no file referenced, use encryptedFile")
		}
		filename, err = d.Decrypt()
		if err != nil {
			return err
		}
	}
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return err
	}
	return nil
}

func GetX509KeyPair(certFile, keyFile, keyPassword string) (tls.Certificate, error) {
	if err := CheckFileExists(certFile); err != nil {
		return tls.Certificate{}, fmt.Errorf("error with certificate file %s: %w", certFile, err)
	}

	b, err := ioutil.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}

	pemBlocks, pkey, err := readAndDecryptPEM(b, keyPassword)

	// If private key not in the cert file, we look for it in the key file
	if pkey == nil {
		pkey, err = getPrivateKey(keyFile, keyPassword)
		if err != nil {
			return tls.Certificate{}, err
		}
	}
	return tls.X509KeyPair(pem.EncodeToMemory(pemBlocks[0]), pkey)
}

// getPrivateKey attempts to load and decrypt the private key if needed
func getPrivateKey(keyFile, keyPassword string) ([]byte, error) {
	if err := CheckFileExists(keyFile); err != nil {
		return nil, fmt.Errorf("error with key file %s: %w", keyFile, err)
	}
	b, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	_, pkey, err := readAndDecryptPEM(b, keyPassword)
	return pkey, err
}

// readAndDecryptPEM reads PEM data and attempts to decrypt if a private key is found encrypted
// using ssl.keyPassword provided in the config
func readAndDecryptPEM(data []byte, keyPassword string) ([]*pem.Block, []byte, error) {
	var pemBlocks []*pem.Block
	var v *pem.Block
	var pkey []byte

	for {
		v, data = pem.Decode(data)
		if v == nil {
			break
		}
		if v.Type == "RSA PRIVATE KEY" {
			if x509.IsEncryptedPEMBlock(v) {
				pass, err := getKeyPassword(keyPassword)
				if err != nil {
					return nil, nil, err
				}
				pkey, _ = x509.DecryptPEMBlock(v, []byte(pass))
				pkey = pem.EncodeToMemory(&pem.Block{
					Type:  v.Type,
					Bytes: pkey,
				})
			} else {
				pkey = pem.EncodeToMemory(v)
			}
		} else {
			pemBlocks = append(pemBlocks, v)
		}
	}
	return pemBlocks, pkey, nil
}

func getKeyPassword(keyPassword string) (string, error) {
	if secrets.IsEncryptedSecret(keyPassword) {
		d, err := secrets.NewDecrypter(context.TODO(), keyPassword)
		if err != nil {
			return "", err
		}
		return d.Decrypt()
	}
	if keyPassword == "" {
		return "", fmt.Errorf("encrypted pem found but no password provided")
	}
	return keyPassword, nil
}
