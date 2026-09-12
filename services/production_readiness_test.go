// Package services production_readiness_test.go
package services

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// TestPaymentPrivateKeyType 验证微信支付只接受 RSA 私钥。
func TestPaymentPrivateKeyType(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		key  any
		want bool
	}{{"RSA", rsaKey, true}, {"ECDSA", ecKey, false}} {
		t.Run(item.name, func(t *testing.T) {
			der, err := x509.MarshalPKCS8PrivateKey(item.key)
			if err != nil {
				t.Fatal(err)
			}
			encoded := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
			if validRSAPrivateKey(encoded) != item.want {
				t.Fatal("支付私钥类型校验错误")
			}
		})
	}
}

// TestValidHTTPSRoot 验证生产资源地址只接受无参数 HTTPS 根地址。
func TestValidHTTPSRoot(t *testing.T) {
	for value, want := range map[string]bool{
		"https://cdn.example.com":     true,
		"http://cdn.example.com":      false,
		"https://cdn.example.com?a=1": false,
		"":                            false,
	} {
		if got := validHTTPSRoot(value); got != want {
			t.Fatalf("validHTTPSRoot(%q)=%v", value, got)
		}
	}
}
