package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/hex"
)

// Portable tt-data envelope. The prefix and key material are algorithm constants
// recovered as a pure-Go reconstruction of the registration wrapper, not device secrets.
var (
	ttEncryptConstant, _ = hex.DecodeString("4dd4c2e6b83162090e52b3c7a6733ba41cb2462b829ab58a196b39db57177524f49baf7f08e8d68d26a72e37c1a95a2f1f05a51892aef2949732b62a38aadd58")
	ttEncryptPrefix, _   = hex.DecodeString("3176c9b6d71579d7e82179c9e7cbde787b54d541bad3b15f2e8fed92293fa25a")
)

// TTEncrypt wraps bytes in the tt-data envelope beginning 74 63 05 10 00 00.
func TTEncrypt(input []byte) []byte {
	prefixHash := sha512.Sum512(ttEncryptPrefix)
	keyIV := sha512.Sum512(append(prefixHash[:], ttEncryptConstant...))
	digest := sha512.Sum512(input)
	plain := make([]byte, 0, len(digest)+len(input)+aes.BlockSize)
	plain = append(plain, digest[:]...)
	plain = append(plain, input...)
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	for i := 0; i < padding; i++ {
		plain = append(plain, byte(padding))
	}
	block, err := aes.NewCipher(keyIV[:16])
	if err != nil {
		return nil
	}
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, keyIV[16:32]).CryptBlocks(encrypted, plain)
	out := make([]byte, 0, 6+len(ttEncryptPrefix)+len(encrypted))
	out = append(out, 0x74, 0x63, 0x05, 0x10, 0x00, 0x00)
	out = append(out, ttEncryptPrefix...)
	out = append(out, encrypted...)
	return out
}
