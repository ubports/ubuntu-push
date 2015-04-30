/*
 Copyright 2013-2014 Canonical Ltd.

 This program is free software: you can redistribute it and/or modify it
 under the terms of the GNU General Public License version 3, as published
 by the Free Software Foundation.

 This program is distributed in the hope that it will be useful, but
 WITHOUT ANY WARRANTY; without even the implied warranties of
 MERCHANTABILITY, SATISFACTORY QUALITY, or FITNESS FOR A PARTICULAR
 PURPOSE.  See the GNU General Public License for more details.

 You should have received a copy of the GNU General Public License along
 with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package testing

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
)

// key&cert generated with go run /usr/lib/go/src/pkg/crypto/tls/generate_cert.go -ca -host push-delivery -rsa-bits 512 -duration 87600h
var (
	TestKeyPEMBlock = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBANRU+pZKMNHpMvg549meJ060xQ4HCjrfVq+AeIER9W1pkaknDj8c
hwOWKHTeztcPF/LHVpKPabn+fSNbFlq+SzcCAwEAAQJBAIOO+4xu/3yv/rKqO7C0
Oyqa+pVMa1w60R0AfqmKFQTqiTgevM77uqjpW1+t0hpK20nyj6MUIPaL+9kZgp7t
mnECIQDqw79PXSzudf10XGy9ve5bRazINHxQYgJ7FvlTT6JhdQIhAOeJxq9zcKni
69ueO1ualz0hn8w6uHPsG9FlZ8C+7Jh7AiAWJgebjjfZ+4nA+6NKt2uQct9dOA5u
awC+6ij1ojK4rQIgNEqAbcWDj0qpe8sLms+aEntSjJxCZiPP0IW3XeeApZsCIDwo
x+YyxXQWJlf9L5TNYPRo+KFEdk3Cew0lv6QNs+xe
-----END RSA PRIVATE KEY-----`)

	TestCertPEMBlock = []byte(`-----BEGIN CERTIFICATE-----
MIIBYzCCAQ+gAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw0xNDA4MjkxMjQyMDFaFw0yNDA4MjYxMjQyMDFaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wXDANBgkqhkiG9w0BAQEFAANLADBIAkEA1FT6lkow0eky+Dnj2Z4nTrTF
DgcKOt9Wr4B4gRH1bWmRqScOPxyHA5YodN7O1w8X8sdWko9puf59I1sWWr5LNwID
AQABo1IwUDAOBgNVHQ8BAf8EBAMCAKQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYD
VR0TAQH/BAUwAwEB/zAYBgNVHREEETAPgg1wdXNoLWRlbGl2ZXJ5MAsGCSqGSIb3
DQEBBQNBABtWCdMFkhIO8+oM3vugOWle9WJZ1FCRWD+cMl76mI1lhmNF4lvEZG47
xUjekA1+heU39WpOEzZSybrOdiEaGbI=
-----END CERTIFICATE-----`)

	// key&cert generated with openssl req -x509 -nodes -newkey rsa:2048
	// -multivalue-rdn -sha512 -days 3650 -keyout testing.key -out
	// testing.cert -subj "/O=Acme Co/CN=push-delivery/"
	TestKeyPEMBlock512 = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQC4ySO/avJFWps8
AygUZ0dcylNr1UxZb4QPHuO93OXAkYX5ngw7TjnWIGHjvoLzLzPZCxlrGl7e+M1H
GNZqFT3kFv/XYexp9Cx3MCDy0ZWkK9BAVDTAxMkjSR8ZwRjByQqniilDA/kr92NQ
yaL0GlajsxpmcGMjDM0Dp5QF+inQM48ADJpJl0xlfFwE8CwfVVGM8G/ZtQpBJ3AN
RelEG1iF8tsT9nVlWF37Zp9Wp/CxDDVTuzboZx9pkryOeJmm0l93x1aoSy6DTVyg
zjdAOjKFjSsjY7we7x7GgpHuUtXymVH7OHdc0ji5+2O+yf9VEDxuym0fJJEgVLfX
ungSHFxJAgMBAAECggEAeC2gyTqF7KM7+LDY3UQ6Plf8H1KvAC+txKPDXFURO8ep
SaoHrH540RFoeNULl5uobc1xL54L+5n27/lwYbgE85YduHegaVx7mty7YRD78LTq
ERxy3rhdVEyXJInYTxgwjLwnj8VCxdx0RDOPfpCurnKqhdssLryBjZHsjGKh1RzH
bv5fNrqMhU0uH82cOKXy20uzyVo5zuLwWA+PxCEeOTMumpWgN4PmtMrjUot2t2/q
jVoEkrB3B5Xs/s8OrEv10t90nNQPcKT89Kts/jdmgDNNg/dtILogiD4JshTG8fIB
STUArRDCE0NXOmB0XuXRxk8YlZyBj2AsIUQcFRrOgQKBgQDrAkE77wIZcCJRYGxK
KkB5zE5Lei44dKEHU5zIueOflsWFC+RZGWVn1+hTQw9Sk1kqm5atrDbfMZDOk62U
bNcQLT+QqDRo3iSYLo9Q5hFNNxMGUm6RMHApr5iIZeoBFDZ7b4+zCEEFNtYukvjY
DWyeTgUqftoOTDebHbHrk9w/0QKBgQDJSnnestarqjLXyF4RWzcFTsDjFgRv53Cq
WrpiQUkk5JLlKliwoTAGTxzH2skJofT6OAQjrc5489mc5Gt6TVwWB49l+OzzG4H/
QSe5X9I5BEEcdD27wDwsaO/NsusM9jZ4IjauTKR5XqGoepbrWrm7+lBgEe1DvBWx
C71U7Eoq+QKBgQDNJT2+zMf/XrSGZu6A21tHN0KNfo2EeMLsu19clXCPKjUoDBZ8
dL/ho0bKD/r7MWcf24vv9So9MW5f9egLbeta0rTvWPXPKUO2mMZAb2VhCxePaDve
f/MZYJB9WMGpyXQ50kwVk7n2jETxiRiyuR09H4xA6VT+MChGPujGZV9ZUQKBgH7i
06/uTCQqRaKAS8vlE+nkmvKLDoD8A6lfR95oCROYgoCzEPVGpl9Tv3C8Gb5YuXSB
mxpilaTpEmQ0GQwfd8zrNxmwsK0OygN9ruzL2ljWtbSaEdAofcYA4Clqf4DMM8nG
x3FYHtXjMURjAn+Z0TsNr1zf8BCin4nbPJ4r1RUBAoGBALFHLtEWwVxpm3MN4f08
GtH2Phd289H0s5SaX/NaWYy44T+Q/d7LuYk72LWX1jZB/2V3OhiFzih0uK44PBM4
Gaiu8c/vl+M1hixeOenTrapE4ORaYt76INIEC8JpqEvGi0DYkUH1D4F8zzAiejgF
t+nz90UBRCRA8vtZ8fiwz8O0
-----END PRIVATE KEY-----`)

	TestCertPEMBlock512 = []byte(`-----BEGIN CERTIFICATE-----
MIIDJzCCAg+gAwIBAgIJAP9ScfFaKlalMA0GCSqGSIb3DQEBDQUAMCoxEDAOBgNV
BAoMB0FjbWUgQ28xFjAUBgNVBAMMDXB1c2gtZGVsaXZlcnkwHhcNMTUwNDE1MTYx
MDM1WhcNMjUwNDEyMTYxMDM1WjAqMRAwDgYDVQQKDAdBY21lIENvMRYwFAYDVQQD
DA1wdXNoLWRlbGl2ZXJ5MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
uMkjv2ryRVqbPAMoFGdHXMpTa9VMWW+EDx7jvdzlwJGF+Z4MO0451iBh476C8y8z
2QsZaxpe3vjNRxjWahU95Bb/12HsafQsdzAg8tGVpCvQQFQ0wMTJI0kfGcEYwckK
p4opQwP5K/djUMmi9BpWo7MaZnBjIwzNA6eUBfop0DOPAAyaSZdMZXxcBPAsH1VR
jPBv2bUKQSdwDUXpRBtYhfLbE/Z1ZVhd+2afVqfwsQw1U7s26GcfaZK8jniZptJf
d8dWqEsug01coM43QDoyhY0rI2O8Hu8exoKR7lLV8plR+zh3XNI4uftjvsn/VRA8
bsptHySRIFS317p4EhxcSQIDAQABo1AwTjAdBgNVHQ4EFgQUG2Qk9GbWWfSPXRTE
+cfOZMljydAwHwYDVR0jBBgwFoAUG2Qk9GbWWfSPXRTE+cfOZMljydAwDAYDVR0T
BAUwAwEB/zANBgkqhkiG9w0BAQ0FAAOCAQEAUw36s8n8a39ECYUmSS5o+PdjmF1v
6K6ld5n7IlFVwCtA1Rkz2L2AUrko/ao1/ZgKhHsIBFQ7mm5fkvuNd14ZEJ0F8LyI
55Et63IYWYOPHl0oNmzTHex0WRL9nmNvxbQ5UytzGTE5amv/sZTOYH9qnpEes68O
TPP+C3OoM+U6hjOXNGG73zb54JHQUZ4arMg2gbVzxNXU2ReoKYKrYexGGuqIlHcE
XdOQp93oJfqWAj111YS6tIn63ccjx7bKzFzaufuVvCIsk0WrXG2rpuqx+0OYzRKc
deU3hnONgWVXjCQdNysBzUXLeOWcv1KuqScETvGZe7D1UIk7HWsAgnQnYQ==
-----END CERTIFICATE-----`)

	// key&cert, same as server/acceptance/ssl/testing.*
	TestKeyPEMBlockAcceptance []byte

	TestCertPEMBlockAcceptance []byte
)

// test tls server & client configs
var (
	TestTLSServerConfigs                     = map[string]*tls.Config{}
	TestTLSClientConfigs                     = map[string]*tls.Config{}
	TestTLSServerConfig, TestTLSClientConfig *tls.Config
)

func init() {
	var err error
	TestKeyPEMBlockAcceptance, err = ioutil.ReadFile(SourceRelative("../server/acceptance/ssl/testing.key"))
	if err != nil {
		panic(err)
	}

	TestCertPEMBlockAcceptance, err = ioutil.ReadFile(SourceRelative("../server/acceptance/ssl/testing.cert"))
	if err != nil {
		panic(err)
	}

	for _, cfgBits := range []struct {
		label string
		key   []byte
		cert  []byte
	}{
		{"sha1", TestKeyPEMBlock, TestCertPEMBlock},
		{"sha512", TestKeyPEMBlock512, TestCertPEMBlock512},
		{"acceptance", TestKeyPEMBlockAcceptance, TestCertPEMBlockAcceptance},
	} {
		cert, err := tls.X509KeyPair(cfgBits.cert, cfgBits.key)
		if err != nil {
			panic(err)
		}
		tlsServerConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		cp := x509.NewCertPool()
		ok := cp.AppendCertsFromPEM(cfgBits.cert)
		if !ok {
			panic("failed to parse test cert")
		}
		tlsClientConfig := &tls.Config{
			RootCAs:    cp,
			ServerName: "push-delivery",
		}
		TestTLSClientConfigs[cfgBits.label] = tlsClientConfig
		TestTLSServerConfigs[cfgBits.label] = tlsServerConfig
	}
	TestTLSClientConfig = TestTLSClientConfigs["sha1"]
	TestTLSServerConfig = TestTLSServerConfigs["sha1"]
}
