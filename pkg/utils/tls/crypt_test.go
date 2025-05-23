// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/secretflow/kuscia/tests/util"
)

func TestTokenCrypt(t *testing.T) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	pubKey, err := ParseRSAPublicKey(EncodePKCS1PublicKey(priKey))
	assert.NoError(t, err)

	originToken := make([]byte, 16)
	_, err = rand.Read(originToken)
	assert.NoError(t, err)
	text, err := EncryptPKCS1v15(pubKey, originToken, []byte{0})
	assert.NoError(t, err)

	decodedToken, err := DecryptPKCS1v15(priKey, text, 16, []byte{0})
	assert.NoError(t, err)
	assert.Equal(t, originToken, decodedToken)
}

func TestCryptAndDecrypt(t *testing.T) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	pubKey, err := ParseRSAPublicKey(EncodePKCS1PublicKey(priKey))
	assert.NoError(t, err)

	evaluateCryptAndDeCrypt(t, priKey, pubKey, strings.Repeat("a", 174))
	evaluateCryptAndDeCrypt(t, priKey, pubKey, strings.Repeat("a", 2048))
	evaluateCryptAndDeCrypt(t, priKey, pubKey, strings.Repeat("a", 4099))
}

func TestVerifyEncodeCert(t *testing.T) {
	encodedCert := util.MakeBase64EncodeCert(t)
	err := VerifyEncodeCert(encodedCert)
	assert.NoError(t, err)
}

func TestVerifyEncodeCertWithoutCert(t *testing.T) {
	err := VerifyEncodeCert("mockcertencode")
	assert.Error(t, err)
}

func evaluateCryptAndDeCrypt(t *testing.T, priKey *rsa.PrivateKey, pubKey *rsa.PublicKey, text string) {
	ciphertext, err := EncryptOAEP(pubKey, []byte(text))
	assert.NoError(t, err)

	plaintext, err := DecryptOAEP(priKey, ciphertext)
	assert.NoError(t, err)
	assert.Equal(t, text, string(plaintext))
}

func TestParsePKCS1PrivateKey(t *testing.T) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	keyData := EncodePKCS1PrivateKey(priKey)
	_, err = ParseKey(keyData, "")
	assert.NoError(t, err)

	pubkeyData := EncodePKCS1PublicKey(priKey)
	pubkey, err := ParseRSAPublicKey(pubkeyData)
	assert.NoError(t, err)

	token := "kuscia123"
	prefix := "test"
	ciphertext, err := EncryptPKCS1v15(pubkey, []byte(token), []byte(prefix))
	assert.NoError(t, err)
	plaintext, err := DecryptPKCS1v15(priKey, ciphertext, len(token), []byte(prefix))
	assert.NoError(t, err)
	assert.Equal(t, token, string(plaintext))
}

func TestParsePKCS8PrivateKey(t *testing.T) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	keyData, err := EncodePKCS8PrivateKey(priKey)
	assert.NoError(t, err)
	_, err = ParseKey(keyData, "")
	assert.NoError(t, err)

	pubkeyData, err := EncodePKCS8PublicKey(priKey)
	assert.NoError(t, err)
	pubkey, err := ParseRSAPublicKey(pubkeyData)
	assert.NoError(t, err)

	token := "kuscia123"
	prefix := "test"
	ciphertext, err := EncryptPKCS1v15(pubkey, []byte(token), []byte(prefix))
	assert.NoError(t, err)
	plaintext, err := DecryptPKCS1v15(priKey, ciphertext, len(token), []byte(prefix))
	assert.NoError(t, err)
	assert.Equal(t, token, string(plaintext))
}
