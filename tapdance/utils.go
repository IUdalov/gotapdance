package tapdance

import (
	"github.com/agl/ed25519/extra25519"
	"golang.org/x/crypto/curve25519"
	"encoding/binary"
	"crypto/cipher"
	"crypto/sha256"
	"crypto/rand"
	"crypto/aes"

	"fmt"

	"time"
	"bytes"
)

func GenerateDecoyAddress() (hostname string, port int) {
	port = 443
	//hostname = "54.85.9.24" // ecen5032.org
	hostname = "tapdance1.freeaeskey.xyz"
	return
}

func AesGcmEncrypt(plaintext []byte, key []byte, iv []byte) (ciphertext []byte, err error) {
	// The key argument should be the AES key, either 16 or 32 bytes
	// to select AES-128 or AES-256.
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesGcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	ciphertext = aesGcmCipher.Seal(nil, iv, plaintext, nil)
	return
}

func AesGcmDecrypt(ciphertext []byte, key []byte, iv []byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return
	}

	aesGcmCipher, err := cipher.NewGCM(block)
	if err != nil {
		return
	}

	plaintext, err = aesGcmCipher.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return
	}
	return
}


func obfuscateTag(stegoPayload []byte, stationPubkey [32]byte) (tag []byte, err error) {
	var sharedSecret, clientPrivate, clientPublic, representative [32]byte
	for ok := false; ok != true; {
		var slice_key_private []byte = clientPrivate[:]
		rand.Read(slice_key_private)

		clientPrivate[0] &= 248
		clientPrivate[31] &= 127
		clientPrivate[31] |= 64

		ok = extra25519.ScalarBaseMult(&clientPublic, &representative, &clientPrivate)
	}

	curve25519.ScalarMult(&sharedSecret, &clientPrivate, &stationPubkey)

	tagBuf := new(bytes.Buffer) // What we have to encrypt with the shared secret using AES
	tagBuf.Write(representative[:])

	stationPubkeyHash := sha256.Sum256(sharedSecret[:])
	aesKey := stationPubkeyHash[:16]
	aesIv := stationPubkeyHash[16:28]

	data := make([]byte, 2 + len(stegoPayload))
	binary.BigEndian.PutUint16(data, uint16(len(stegoPayload)))
	copy(data[2:], stegoPayload)

	encryptedData, err := AesGcmEncrypt(data, aesKey, aesIv)
	if err != nil {
		return
	}

	tagBuf.Write(encryptedData)
	tag = tagBuf.Bytes()
	Logger.Debugf("len(tag)", tagBuf.Len())
	return
}

func reverseEncrypt(ciphertext []byte, keyStream []byte) (plaintext string) {
	// our plaintext can be antyhing where x & 0xc0 == 0x40
	// i.e. 64-127 in ascii (@, A-Z, [\]^_`, a-z, {|}~ DEL)
	// This means that we are allowed to choose the last 6 bits
	// of each byte in the ciphertext arbitrarily; the upper 2
	// bits will have to be 01, so that our plaintext ends up
	// in the desired range.
	var ka, kb, kc, kd byte    // key stream bytes
	var ca, cb, cc, cd byte    // ciphertext bytes
	var pa, pb, pc, pd byte    // plaintext bytes
	var sa, sb, sc byte        // secret bytes

	var tag_idx, keystream_idx int

	for tag_idx < len(ciphertext) {
		ka = keyStream[keystream_idx]
		kb = keyStream[keystream_idx + 1]
		kc = keyStream[keystream_idx + 2]
		kd = keyStream[keystream_idx + 3]
		keystream_idx += 4

		// read 3 bytes
		sa = ciphertext[tag_idx]
		sb = ciphertext[tag_idx + 1]
		sc = ciphertext[tag_idx + 2]
		tag_idx += 3

		// figure out what plaintext needs to be in base64 encode
		ca = (ka & 0xc0) | ((sa & 0xfc) >> 2)                        // 6 bits sa
		cb = (kb & 0xc0) | (((sa & 0x03) << 4) | ((sb & 0xf0) >> 4)) // 2 bits sa, 4 bits sb
		cc = (kc & 0xc0) | (((sb & 0x0f) << 2) | ((sc & 0xc0) >> 6)) // 4 bits sb, 2 bits sc
		cd = (kd & 0xc0) | (sc & 0x3f)                               // 6 bits sc

		// Xor with key_stream, and add on 0x40 so it's in range of allowed
		pa = (ca ^ ka) + 0x40
		pb = (cb ^ kb) + 0x40
		pc = (cc ^ kc) + 0x40
		pd = (cd ^ kd) + 0x40

		plaintext += string(pa)
		plaintext += string(pb)
		plaintext += string(pc)
		plaintext += string(pd)
	}
	return
}

func timeMs() int64 {
	return time.Now().UnixNano() / (int64(time.Millisecond)/int64(time.Nanosecond))
}

func printHex(byteArray []byte, name string) {
	fmt.Print(name, ": [")
	for i := 0; i < len(byteArray); i++ {
		if byteArray[i] >= 0x10 {
			//fmt.Printf("%x", byte_array[i])
			fmt.Printf("%v, ", byteArray[i])
		} else {
		//	fmt.Printf("0%x", byte_array[i])
			fmt.Printf("%v, ", byteArray[i])
		}
	}
	fmt.Println("]")
}
