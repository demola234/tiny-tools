package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const (
	certFile  = "rootCA.pem"
	keyFile   = "rootCA-key.pem"
	leafDays  = 397
	caYears   = 10
	clockSkew = time.Hour
)

type CA struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	pem  []byte
}

func (ca *CA) PEM() []byte { return ca.pem }

func LoadOrCreate(dir string, now time.Time) (*CA, bool, error) {
	certPath, keyPath := filepath.Join(dir, certFile), filepath.Join(dir, keyFile)
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	switch {
	case certErr == nil && keyErr == nil:
		ca, err := load(certPath, keyPath)
		return ca, false, err
	case errors.Is(certErr, fs.ErrNotExist) && errors.Is(keyErr, fs.ErrNotExist):
		ca, err := create(dir, certPath, keyPath, now)
		return ca, err == nil, err
	}
	return nil, false, fmt.Errorf("the local CA in %s is incomplete; delete the folder to make a new one", dir)
}

func load(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	certBlock, _ := pem.Decode(certPEM)
	keyBlock, _ := pem.Decode(keyPEM)
	if certBlock == nil || keyBlock == nil {
		return nil, fmt.Errorf("%s or %s isn't a PEM file; delete them to make a new CA", certPath, keyPath)
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", certPath, err)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", keyPath, err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("%s doesn't hold a signing key", keyPath)
	}
	return &CA{Cert: cert, Key: signer, pem: certPEM}, nil
}

func serial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func create(dir, certPath, keyPath string, now time.Time) (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	sn, err := serial()
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: "MockMachina local CA", Organization: []string{"MockMachina"}},
		NotBefore:             now.Add(-clockSkew),
		NotAfter:              now.AddDate(caYears, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &CA{Cert: cert, Key: key, pem: certPEM}, nil
}

func (ca *CA) Leaf(names []string, now time.Time) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	sn, err := serial()
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{Organization: []string{"MockMachina development certificate"}},
		NotBefore:    now.Add(-clockSkew),
		NotAfter:     now.AddDate(0, 0, leafDays),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, n := range names {
		if ip := net.ParseIP(n); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, n)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.Cert, &key.PublicKey, ca.Key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der, ca.Cert.Raw}, PrivateKey: key, Leaf: leaf}, nil
}

func Names(addrs []net.Addr, extra ...string) []string {
	names := []string{"localhost", "127.0.0.1", "::1", "10.0.2.2"}
	add := func(n string) {
		if n != "" && !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.IsLinkLocalUnicast() || ipnet.IP.To4() == nil {
			continue
		}
		add(ipnet.IP.String())
	}
	for _, n := range extra {
		add(n)
	}
	return names
}
