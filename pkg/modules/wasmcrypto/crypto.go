package wasmcrypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"math/big"
)

var sha256DigestInfoPrefix = []byte{
	0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48,
	0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20,
}

var sha256K = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5,
	0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
	0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc,
	0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
	0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
	0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3,
	0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5,
	0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
	0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}

// Sum256 computes SHA-256 without relying on crypto/sha256.
// This avoids TinyGo + Go 1.25 stdlib compatibility issues in WASM modules.
func Sum256(msg []byte) [32]byte {
	bitLen := uint64(len(msg)) * 8

	padded := make([]byte, len(msg)+1)
	copy(padded, msg)
	padded[len(msg)] = 0x80

	for len(padded)%64 != 56 {
		padded = append(padded, 0)
	}

	padded = append(padded,
		byte(bitLen>>56),
		byte(bitLen>>48),
		byte(bitLen>>40),
		byte(bitLen>>32),
		byte(bitLen>>24),
		byte(bitLen>>16),
		byte(bitLen>>8),
		byte(bitLen),
	)

	h0 := uint32(0x6a09e667)
	h1 := uint32(0xbb67ae85)
	h2 := uint32(0x3c6ef372)
	h3 := uint32(0xa54ff53a)
	h4 := uint32(0x510e527f)
	h5 := uint32(0x9b05688c)
	h6 := uint32(0x1f83d9ab)
	h7 := uint32(0x5be0cd19)

	var w [64]uint32
	for chunk := 0; chunk < len(padded); chunk += 64 {
		for i := 0; i < 16; i++ {
			off := chunk + i*4
			w[i] = uint32(padded[off])<<24 | uint32(padded[off+1])<<16 | uint32(padded[off+2])<<8 | uint32(padded[off+3])
		}

		for i := 16; i < 64; i++ {
			s0 := rightRotate(w[i-15], 7) ^ rightRotate(w[i-15], 18) ^ (w[i-15] >> 3)
			s1 := rightRotate(w[i-2], 17) ^ rightRotate(w[i-2], 19) ^ (w[i-2] >> 10)
			w[i] = w[i-16] + s0 + w[i-7] + s1
		}

		a, b, c, d := h0, h1, h2, h3
		e, f, g, h := h4, h5, h6, h7

		for i := 0; i < 64; i++ {
			s1 := rightRotate(e, 6) ^ rightRotate(e, 11) ^ rightRotate(e, 25)
			ch := (e & f) ^ (^e & g)
			temp1 := h + s1 + ch + sha256K[i] + w[i]
			s0 := rightRotate(a, 2) ^ rightRotate(a, 13) ^ rightRotate(a, 22)
			maj := (a & b) ^ (a & c) ^ (b & c)
			temp2 := s0 + maj

			h = g
			g = f
			f = e
			e = d + temp1
			d = c
			c = b
			b = a
			a = temp1 + temp2
		}

		h0 += a
		h1 += b
		h2 += c
		h3 += d
		h4 += e
		h5 += f
		h6 += g
		h7 += h
	}

	var digest [32]byte
	putU32BE(digest[0:4], h0)
	putU32BE(digest[4:8], h1)
	putU32BE(digest[8:12], h2)
	putU32BE(digest[12:16], h3)
	putU32BE(digest[16:20], h4)
	putU32BE(digest[20:24], h5)
	putU32BE(digest[24:28], h6)
	putU32BE(digest[28:32], h7)

	return digest
}

// SignRS256PKCS1v15 signs a SHA-256 digest using RSA PKCS#1 v1.5 (RS256)
// without calling crypto/rsa's hash-aware path.
func SignRS256PKCS1v15(priv *rsa.PrivateKey, digest [32]byte) ([]byte, error) {
	if priv == nil || priv.N == nil || priv.D == nil {
		return nil, fmt.Errorf("invalid RSA private key")
	}

	k := (priv.N.BitLen() + 7) / 8
	tLen := len(sha256DigestInfoPrefix) + len(digest)
	if k < tLen+11 {
		return nil, fmt.Errorf("RSA modulus too small")
	}

	em := make([]byte, k)
	em[0] = 0x00
	em[1] = 0x01
	psLen := k - tLen - 3
	for i := 0; i < psLen; i++ {
		em[2+i] = 0xff
	}
	em[2+psLen] = 0x00
	copy(em[3+psLen:], sha256DigestInfoPrefix)
	copy(em[3+psLen+len(sha256DigestInfoPrefix):], digest[:])

	m := new(big.Int).SetBytes(em)
	if m.Cmp(priv.N) >= 0 {
		return nil, fmt.Errorf("encoded message too large")
	}

	s := new(big.Int).Exp(m, priv.D, priv.N)
	sig := make([]byte, k)
	s.FillBytes(sig)
	return sig, nil
}

// SignES256P256 signs a SHA-256 digest using ECDSA over P-256 and returns
// the compact JWS signature form: R(32) || S(32).
func SignES256P256(priv *ecdsa.PrivateKey, digest [32]byte) ([]byte, error) {
	if priv == nil || priv.Curve == nil || priv.D == nil || priv.X == nil || priv.Y == nil {
		return nil, fmt.Errorf("invalid ECDSA private key")
	}

	params := priv.Params()
	if params == nil || params.N == nil || params.Name != "P-256" {
		return nil, fmt.Errorf("unsupported ECDSA curve: expected P-256")
	}

	if priv.Curve != elliptic.P256() {
		return nil, fmt.Errorf("unsupported ECDSA curve implementation")
	}

	n := params.N
	one := big.NewInt(1)
	nMinusOne := new(big.Int).Sub(new(big.Int).Set(n), one)
	e := new(big.Int).SetBytes(digest[:])
	halfN := new(big.Int).Rsh(new(big.Int).Set(n), 1)

	for attempts := 0; attempts < 128; attempts++ {
		k, err := rand.Int(rand.Reader, nMinusOne)
		if err != nil {
			return nil, fmt.Errorf("failed to generate nonce: %w", err)
		}
		k.Add(k, one)

		x1, _ := priv.Curve.ScalarBaseMult(k.Bytes())
		if x1 == nil {
			continue
		}

		r := new(big.Int).Mod(x1, n)
		if r.Sign() == 0 {
			continue
		}

		kInv := new(big.Int).ModInverse(k, n)
		if kInv == nil {
			continue
		}

		s := new(big.Int).Mul(priv.D, r)
		s.Add(s, e)
		s.Mul(s, kInv)
		s.Mod(s, n)
		if s.Sign() == 0 {
			continue
		}

		if s.Cmp(halfN) > 0 {
			s.Sub(n, s)
		}

		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
		return sig, nil
	}

	return nil, fmt.Errorf("failed to generate ECDSA signature")
}

// HMACSHA256 computes HMAC-SHA256 without relying on crypto/hmac.
func HMACSHA256(key, data []byte) []byte {
	const blockSize = 64

	if len(key) > blockSize {
		sum := Sum256(key)
		key = sum[:]
	}

	k0 := make([]byte, blockSize)
	copy(k0, key)

	ipad := make([]byte, blockSize)
	opad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		ipad[i] = k0[i] ^ 0x36
		opad[i] = k0[i] ^ 0x5c
	}

	inner := make([]byte, 0, blockSize+len(data))
	inner = append(inner, ipad...)
	inner = append(inner, data...)
	innerSum := Sum256(inner)

	outer := make([]byte, 0, blockSize+32)
	outer = append(outer, opad...)
	outer = append(outer, innerSum[:]...)
	outerSum := Sum256(outer)

	out := make([]byte, 32)
	copy(out, outerSum[:])
	return out
}

func rightRotate(x uint32, n uint) uint32 {
	return (x >> n) | (x << (32 - n))
}

func putU32BE(dst []byte, v uint32) {
	dst[0] = byte(v >> 24)
	dst[1] = byte(v >> 16)
	dst[2] = byte(v >> 8)
	dst[3] = byte(v)
}
