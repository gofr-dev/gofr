package kafka

import (
	"os"
	"path/filepath"
	"testing"
)

func createTempFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "test_cert_*.pem")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func Test_CreateTLSConfig_Success(t *testing.T) {
	caFile, clientCertFile, clientKeyFile := createTestFiles(t)
	defer os.Remove(caFile)
	defer os.Remove(clientCertFile)
	defer os.Remove(clientKeyFile)

	tests := []struct {
		name     string
		tlsConf  TLSConfig
		wantCert bool
	}{
		{"Only CA Cert", TLSConfig{CACertFile: caFile}, false},
		{"CA Cert + Client Cert/Key", TLSConfig{CACertFile: caFile, CertFile: clientCertFile, KeyFile: clientKeyFile}, true},
		{"Only Client Cert/Key", TLSConfig{CertFile: clientCertFile, KeyFile: clientKeyFile}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateTLSConfig(t, &tt.tlsConf, tt.wantCert)
		})
	}
}

func createTestFiles(t *testing.T) (caFile, clientCertFile, clientKeyFile string) {
	t.Helper()

	// Valid CA Certificate
	caCertContent := `-----BEGIN CERTIFICATE-----
MIICojCCAYoCCQDUMM9AFXkWaDANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDDAhL
YWZrYS1DQTAeFw0yNTAzMTcwODU5NDFaFw0yNjAzMTcwODU5NDFaMBMxETAPBgNV
BAMMCEthZmthLUNBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzyxH
lWoFiwBnB7bJjS8AL6liIpWLuxxNugPYWUn/uodwKF4wM1lXlY70D85cPewOtYm7
gw/NsBYgz10jtla/7IFHkOoCTx5L9NkAI79i2jl12cG3oCKCgGocYafGQZBODuEI
UjnXOlvkYbaqSj+FE8wyAAxMqROLN48Iw8W0UkwfAIKMEbf75o1/828wysMbPxzh
nD2G/g/sF8zmi++kPjYkeKMhOVIl+EWf6nrpzO9GoTwKyIntOu4xPZM1A0icCgL2
3JNbMiuR4tLFvL1BHzwknF/8nucWu6g4S9XS/Ql57mo7FS1LtNvksPLwEKw2Ll/u
QV7pnVgBgszH+z5ubwIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQA4RqW6N5BO4gkS
j7qRiVEx8NoYx9kMvfmuN7ldjqA00gb8DI4tAIUZ+G8exfIVLLJL86L2Gis8y7yC
W9RCuQnuWc5Co+wTshGEBOn7WA1LIWGYnZDnxrhZ2Hv5HQVw48OCp0R1/YCD7Xt2
OLXk3tA2sG5rVmjOOhN+wYefwTioedmnZbiIn50MBTHQ0cad0Rfcl4iEDV0o1idi
HQ7R+FdTHWSfjfrYmbBDB6ALNwWXgEMdHTv3SShJgxcdbtfgLJwtIFjIzg9MRRx+
izrooAIzjs9GHmZVaoQsbyG27OPH6v4SqMwvPQttfmTKFY9aNGlmr7hfVUXdnGUd
+FR0EKOA
-----END CERTIFICATE-----`

	// Valid Client Certificate and Key
	clientCertContent := `-----BEGIN CERTIFICATE-----
MIICoDCCAYgCCQD04XTuhKncLDANBgkqhkiG9w0BAQsFADATMREwDwYDVQQDDAhL
YWZrYS1DQTAeFw0yNTAzMTcwOTAwMjZaFw0yNjAzMTcwOTAwMjZaMBExDzANBgNV
BAMMBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALAFJrUQ
qboZ5WFTkCbrUFw3NnbshVZZecYNzxEUXc94k4ZA58PrEkVmt78VBJyrFlRxTWCu
NT98VFdNZLR58yd4R48Q5wdrfBnkhw2byc1n19cgW+KOkyEGDhkycwc48GxTe6BB
EP/ntTPfBevtuLVYrJIiFyWiy+opBDv+by+A9XsASkbWxui99/HKWDswuiAwDsp1
q589lGIL3c4Uj9L6sGxA8ekMJAFM/Rq1SMxcV172E68HifR6/nJcfZU3/JKnVjN1
WX1lOrc9ENiY8pzoWU63ZONv6KSWXPxB1sW/HR1QFpFAiuwDh2xFcZ2zghMEuxmK
rlZT7tDuw6DOyL8CAwEAATANBgkqhkiG9w0BAQsFAAOCAQEAgm+2Gr2LD7eL7UGH
R8lwaKxyA0gvRA0II9T3s4ReaCzXmDCgQl9YFs2QyyxuAPRhzVIcrQCo9IsN6+eN
vkoUsePb9m7onMNoKzJ63EhbsDnkj3eirG33lLaB5cG/TJm/8z+cm80HJ0viWtea
/vgG7Gzdbkkr5Swl4F9IHI/DpztMf2u05U68DhBxzs2eI/qY5majMfajzD13jAwe
u1D4NuBnhNYxoMReK62alDiFIIfFy1+/pxux+mQqKo18VpMKo0YwXxrfGOi9+5AD
IFi4oUxW0wLNUmTJFSIrFRE3eYWy56XiI8jPs7U94It8YwjhDSeHwslMKbGwogqI
Om59HA==
-----END CERTIFICATE-----`

	//nolint:gosec // This is a test certificate
	clientKeyContent := `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAsAUmtRCpuhnlYVOQJutQXDc2duyFVll5xg3PERRdz3iThkDn
w+sSRWa3vxUEnKsWVHFNYK41P3xUV01ktHnzJ3hHjxDnB2t8GeSHDZvJzWfX1yBb
4o6TIQYOGTJzBzjwbFN7oEEQ/+e1M98F6+24tViskiIXJaLL6ikEO/5vL4D1ewBK
RtbG6L338cpYOzC6IDAOynWrnz2UYgvdzhSP0vqwbEDx6QwkAUz9GrVIzFxXXvYT
rweJ9Hr+clx9lTf8kqdWM3VZfWU6tz0Q2JjynOhZTrdk42/opJZc/EHWxb8dHVAW
kUCK7AOHbEVxnbOCEwS7GYquVlPu0O7DoM7IvwIDAQABAoIBADtM0PiJL5UZ6lQ6
scLa3gzjMP8pudYYeNUHi+4mHWCrL5A4R5ySkmo9K8Q9UXtyjChQr4/VwOytd0Ce
O0IuH4P5mqoROLQgOwQCIJmuFXOU+3tnVG1kOR8UCiXlACm7vgvQqEKaCR8dscdS
6IzOXr8Bq8njoEa2rNorjVik5FJtIbbBEJJI9nISG7901pXk+kvEXujX4Tt1xqsF
W5L6Q+2rM2gCedQr+11aml265KdJq8sYGFf8/PNm9AG5V7xRKJWyFzohGuuMimRM
WSb0uEIdkfNfaxfYc/2g5u8g9mPyzAdAjsd9tl3nxdHdCou1B6XIeDRYJupZ7z18
JZAcMmECgYEA2LlmNlQX9OWoc8+lww/43LgU6KVpIBUtxFSydqSdYJVFk33tKtyq
mD1IlSLJ9X9kQ9QWVKxtKGudY+AnEsEMua8nm3ypO7LSMDdKrrnBs+H1wbPyixnW
AsB66v3Vmn6mMf1D1PE8GY/JlprZgllq2fJ/D+jVx3BCjYYXP9VdV28CgYEAz+tY
b7dosqXeG2c6pZ/1gYtqde/prclDuIdfMYVYE7RO9mwknK9wnOYYMKJwnSsnsefv
QTrGkbfBflGKcpzf/Ux+MyJkfmQYRtczPS0XmtovkW/UUwTYCTsXk+V+zd+uiX87
2jMURXGg6qpcT9Gm0zQgoeS45iyK2eVWTXhte7ECgYAPyKjmCgfYoSU8kgHri+0+
/fUf4HQgjwpPQy/gLir8DsMLc99jAME35zazDd6Rj56Yxgh+UDR+/h9vV7LgzciE
eXoz+8dDfsmKE2zP/t1ZoXpJijZ+5PnOJ4CMPsJgxxqJh316M7uBzRQMcOiocqSy
jNOuL/Hp3YYrUnm8/2gV5wKBgDhann6xJHR/VoLw6MlpYJ57DiDnJNwQmAVU061V
afj1Pw21Y/r/5jLwfo/4BzPiNYEXzxZL+vQV7SDysua7tE4wRGhRoxFKyfWxcFbd
eO9kwc3WlKLnxjJCTPKuGj9sqB7mWG+ctprX4HiaMikENwY5s7qNhrwESKIkcc7P
nEURAoGARj9QGcxbs5jBAxqRCHp+hzDEArIKqe2aDawAsx09Zv6kd97HiU/DFnJy
s18fZGQi04zDLMx72bYHG/9SdtcKLdwKgQcpHLLwDvAtXXyRE7YTvy6ziChf25lX
Q1k1WBr5rFlCp0GK2DbAkuCrLj0GghVAFYhxN19XRT/Dax1vgFo=
-----END RSA PRIVATE KEY-----`

	var err error

	// Create temporary files
	caFile, err = createTempFile(caCertContent)
	if err != nil {
		t.Fatalf("Failed to create temp CA cert file: %v", err)
	}

	clientCertFile, err = createTempFile(clientCertContent)
	if err != nil {
		t.Fatalf("Failed to create temp client cert file: %v", err)
	}

	clientKeyFile, err = createTempFile(clientKeyContent)
	if err != nil {
		t.Fatalf("Failed to create temp client key file: %v", err)
	}

	return caFile, clientCertFile, clientKeyFile
}

func validateTLSConfig(t *testing.T, tlsConf *TLSConfig, wantCert bool) {
	t.Helper()

	cfg, err := createTLSConfig(tlsConf)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if wantCert && len(cfg.Certificates) == 0 {
		t.Errorf("Expected TLS certificate, but got none")
	}

	if !wantCert && len(cfg.Certificates) > 0 {
		t.Errorf("Expected no TLS certificate, but found one")
	}
}

func TestCreateTLSConfig_Errors(t *testing.T) {
	invalidFile := filepath.Join(os.TempDir(), "nonexistent_file.pem")

	tests := []struct {
		name    string
		tlsConf TLSConfig
	}{
		{"Invalid CA Cert File", TLSConfig{CACertFile: invalidFile}},
		{"Invalid Client Cert File", TLSConfig{CertFile: invalidFile, KeyFile: invalidFile}},
		{"Invalid Client Key File", TLSConfig{CertFile: invalidFile, KeyFile: invalidFile}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createTLSConfig(&tt.tlsConf)
			if err == nil {
				t.Errorf("Expected an error but got none")
			}
		})
	}
}
