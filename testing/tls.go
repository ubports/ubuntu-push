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

// key&cert generated with go run /usr/lib/go/src/pkg/crypto/tls/generate_cert.go -ca -host localhost -rsa-bits 512 -duration 87600h
var (
	TestKeyPEMBlock = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPAIBAAJBAPw+niki17X2qALE2A2AzE1q5dvK9CI4OduRtT9IgbFLC6psqAT2
1NA+QbY17nWSSpyP65zkMkwKXrbDzstwLPkCAwEAAQJAKwXbIBULScP6QA6m8xam
wgWbkvN41GVWqPafPV32kPBvKwSc+M1e+JR7g3/xPZE7TCELcfYi4yXEHZZI3Pbh
oQIhAP/UsgJbsfH1GFv8Y8qGl5l/kmwwkwHhuKvEC87Yur9FAiEA/GlQv3ZfaXnT
lcCFT0aL02O0RDiRYyMUG/JAZQJs6CUCIQCHO5SZYIUwxIGK5mCNxxXOAzyQSiD7
hqkKywf+4FvfDQIhALa0TLyqJFom0t7c4iIGAIRc8UlIYQSPiajI64+x9775AiEA
0v4fgSK/Rq059zW1753JjuB6aR0Uh+3RqJII4dUR1Wg=
-----END RSA PRIVATE KEY-----`)

	TestCertPEMBlock = []byte(`-----BEGIN CERTIFICATE-----
MIIBYzCCAQ+gAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw0xMzEyMTkyMDU1NDNaFw0yMzEyMTcyMDU1NDNaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAPw+niki17X2qALE2A2AzE1q5dvK
9CI4OduRtT9IgbFLC6psqAT21NA+QbY17nWSSpyP65zkMkwKXrbDzstwLPkCAwEA
AaNUMFIwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wGgYDVR0RBBMwEYIJbG9jYWxob3N0hwR/AAABMAsGCSqGSIb3
DQEBBQNBAFqiVI+Km2XPSO+pxITaPvhmuzg+XG3l1+2di3gL+HlDobocjBqRctRU
YySO32W07acjGJmCHUKpCJuq9X8hpmk=
-----END CERTIFICATE-----`)
)
