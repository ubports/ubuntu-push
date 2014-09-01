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
)
