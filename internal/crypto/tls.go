package crypto

import (
	"crypto/tls"
	"crypto/x509"
	"sync"

	"google.golang.org/grpc/credentials"
)

// Embedded self-signed CA certificate and key for gRPC TLS.
// Generated once and reused across server and agent instances.

//go:generate go run tls_gen.go

var (
	tlsCertPEM = `-----BEGIN CERTIFICATE-----
MIIDPDCCAiSgAwIBAgIBATANBgkqhkiG9w0BAQsFADAYMRYwFAYDVQQDEw13YXRj
aGRvZyBnUlBDMB4XDTI2MDcyNjA5NDQxNVoXDTI3MDcyNzA5NDQxNVowGDEWMBQG
A1UEAxMNd2F0Y2hkb2cgZ1JQQzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAMEHPAHURhs+MOAI5g+w9UdhYozuIduYUQO8DIP3m/tpBbPp9sN748hXD0yS
rYjG3PzQIIOW0EJUbcLAOacoKXmZomEggMCSvramEeY5HlN+ugvg8d90u+W+JwwS
BR3rvIyk/0iIDwbadt58ZKztbMEVAeO/Brc1vhvp3lLM5Fw/n83gP94MrOHAW/db
68i2K9SYF3iyvmLfTqzV95qDavjYj+Hrnh6slIjUOIBpWAEZdtDPhQQ6h2geq197
Tx2VKSQVPybePYu1sSFxLhmhzwqg7ww2T/SdoJ/kw3n1S0+SPLbhxz+ztt5BfRF+
EGX/7uYV7Xpngyfj10TfH+jLut0CAwEAAaOBkDCBjTAOBgNVHQ8BAf8EBAMCAqQw
HQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMA8GA1UdEwEB/wQFMAMBAf8w
HQYDVR0OBBYEFBSHxvrrmYvp0JxeW57174h2VMo4MCwGA1UdEQQlMCOCCWxvY2Fs
aG9zdIcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEA
YDfhvlCiEXyhKTAmE+EcLThSp2BEENa7CTHDvTvOJIaDr73VLFwfwfZwZ/gKSxFs
f7m6fk8th2D9gMbkE7QLoO55VwiJTVp0fa4xfmCBaeAIlG5oRxowF2w3IgiEvnve
+4bOZeQjopH504wb+KLkkSUUiA5Gcr2HzhonohVAwzVqQiMSVaHdmyaFA0WpXUGs
lFdQarhDmi7PXXtO7ly7oa5bho8H4cqfk2n4CGzlIkIh89OF91LB13LjeMwOGjFJ
fgC0NciGf80sUyYLDd2x671P5wqztfonjkAAI+sZEgYtm+e4NRaJ/oJb8QEef0qt
dwziRhClUlSBPldxBawn0Q==
-----END CERTIFICATE-----`

	tlsKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAwQc8AdRGGz4w4AjmD7D1R2FijO4h25hRA7wMg/eb+2kFs+n2
w3vjyFcPTJKtiMbc/NAgg5bQQlRtwsA5pygpeZmiYSCAwJK+tqYR5jkeU366C+Dx
33S75b4nDBIFHeu8jKT/SIgPBtp23nxkrO1swRUB478GtzW+G+neUszkXD+fzeA/
3gys4cBb91vryLYr1JgXeLK+Yt9OrNX3moNq+NiP4eueHqyUiNQ4gGlYARl20M+F
BDqHaB6rX3tPHZUpJBU/Jt49i7WxIXEuGaHPCqDvDDZP9J2gn+TDefVLT5I8tuHH
P7O23kF9EX4QZf/u5hXtemeDJ+PXRN8f6Mu63QIDAQABAoIBAB3ffz9vOSxVKx44
8lXiVotl/GkAH5hLEdqomy0/QFIf1kSaRFjLjxx9sL3yg03ELQYpNab3y6JAS75U
nneKpSSPDMzISXTEISTGPcXp+BIG7kcRWI1zFPNAwu1Ayq7vQT5o+KMif2TZoYTc
Ln5+vhKBrEmxUho/hHzwbDpXQE4wOznj5ehEzf8SOcQm9LDbxkzMCJX1+PfOrpuT
N+L0nsqEDMxftE7nm/HYwNCmerCd91C0kyQZGz0pRTacikWFi54Mrcqm58lmpOn/
e+hsZIQo7SDF3bun0O+M+3r/RAm1ah3u3FH2Fqge1OgTO1KdctqFX/NojHk5ek6M
kGmYJxECgYEAxKlJDYlnSqCRYxdxq7tLJqsYwNBuIJ9vwPczaeHR89JU8p0uyc9w
XIKjYKF8vqJz7wmlHzdve6YZfHtWbO4YyJ5R4MaXBw67KEySpeO//PaCAJSbsd6R
uQZSFz2NMr6zWkeSL9fbyxCfHRA8NjfpunwgJHDaCju5+4VckSEZzPUCgYEA+0VS
/V9KxudPSKzPdijknIbKNErwwc/pzqV5EnS4igOfldZPpdBnmR/0x6GOu/879uTq
A3JZzy/drC/ovSGSB5xvIhGPrertYFeqze01JzAJv2OTb76Mt2TsdudXK+NU/QeC
uNDR/JUye+XXvpihxZwRqfjRfEKMvJAf4bQBhUkCgYBrT3hmY5CyXxWWPaewLr4e
NoSGSfWd5YIEiJ9MaoW3BxGFZZGvW3sTb9GYm+XG3DxothmdBBHYJdWIYIDTZcSu
S/2fqp2kozwrDEWFMdaEQTrE+FJQ54MatEE9H0AZ7YdOfvldE+uCTeqU4FQKvc3T
DYI4gD/qD5c3kRjmtGowtQKBgQDlAa+7gSgT1ClsYSPL20VQa4DK3CpFWgsL7cBE
0+CE2PyPgX2h8CkbZAaiE1qVeO/b+5JUhdnYfRWZoyiJh5kiGq8m6755kg26quvf
NvwktSGNL2HmjFKPqwng7MOEGnMREdFQQ/G+NPSH+1kAOvfltHJc6YtzpuvBx9Fm
0bo5EQKBgQC91kdQuesdHnRHjgz/zQfEBsK89S/vn1fdhnAgQlOZpwxV+UnfEwYI
WT0E+hx0ow77A+eV7SS/ZuXROfbdxcuBTg9pwxrLSQCld0ZT2hzdgKyV5NoxRzR6
bqXaMefLjYwLgZoufxsVldfTH+KNL0BHdRlo8dkVPDUKLGdL1QXfdw==
-----END RSA PRIVATE KEY-----`

	// Cached transport credentials (lazy init).
	serverCreds credentials.TransportCredentials
	clientCreds credentials.TransportCredentials
	credsOnce   sync.Once
)

// initCredentials parses the embedded PEM data and builds transport credentials.
func initCredentials() {
	cert, err := tls.X509KeyPair([]byte(tlsCertPEM), []byte(tlsKeyPEM))
	if err != nil {
		panic("crypto: failed to load embedded TLS cert: " + err.Error())
	}

	serverCreds = credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM([]byte(tlsCertPEM)) {
		panic("crypto: failed to parse embedded CA cert")
	}
	clientCreds = credentials.NewTLS(&tls.Config{
		RootCAs:    caPool,
		ServerName: "localhost",
		MinVersion: tls.VersionTLS12,
	})
}

// GRPCServerCredentials returns gRPC transport credentials for the server
func GRPCServerCredentials() credentials.TransportCredentials {
	credsOnce.Do(initCredentials)
	return serverCreds
}

// GRPCClientCredentials returns gRPC transport credentials for the client
func GRPCClientCredentials() credentials.TransportCredentials {
	credsOnce.Do(initCredentials)
	return clientCreds
}
