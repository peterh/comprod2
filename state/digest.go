package state

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"io"
	"log"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/sha3"
)

// newKey must be called under the lock (or in a context
// where the lock is unnecessary)
func (g *GameState) newKey() {
	g.Key = make([]byte, 256/8)
	_, err := io.ReadFull(rand.Reader, g.Key)
	if err != nil {
		log.Fatal(err)
	}
}

func leftEncode(encbuf []byte, value uint64) []byte {
	var n uint

	for v := value; v > 0 && n < 8; v >>= 8 {
		n++
	}
	if n == 0 {
		n = 1
	}
	encbuf = append(encbuf, byte(n))
	for i := uint(1); i <= n; i++ {
		encbuf = append(encbuf, byte(value>>(8*(n-i))))
	}
	return encbuf
}

func rightEncode(encbuf []byte, value uint64) []byte {
	var n uint

	for v := value; v > 0 && (n < 8); v >>= 8 {
		n++
	}
	if n == 0 {
		n = 1
	}
	for i := uint(1); i <= n; i++ {
		encbuf = append(encbuf, byte(value>>(8*(n-i))))
	}
	encbuf = append(encbuf, byte(n))
	return encbuf
}

func KMAC128(separator string, key, data []byte, outBits int) []byte {
	const pad = 168 // key pad length for KMAC128
	hash := sha3.NewCShake128([]byte("KMAC"), []byte(separator))
	buf := leftEncode(nil, pad)
	buf = leftEncode(buf, uint64(len(key))*8)
	hash.Write(buf)
	hash.Write(key)
	if len(key) < pad {
		hash.Write(bytes.Repeat([]byte{0}, pad-len(key)))
	}
	hash.Write(data)
	hash.Write(rightEncode(buf[:0], uint64(outBits)))
	out := make([]byte, (outBits+7)/8)
	hash.Read(out)
	return out
}

func (g *Game) Hash(thing, name string) []byte {
	g.Lock()
	defer g.Unlock()
	return KMAC128(thing, g.g.Key, []byte(name), 160)
}

func GetSeed() (rv int64) {
	binary.Read(rand.Reader, binary.LittleEndian, &rv)
	return
}

var pwdMutex sync.Mutex

func pwdHash(salt []byte, password string) []byte {
	// With this lock, it's easy to DoS the comprod instance.
	// Without this lock, it's easy to DoS the entire machine.
	pwdMutex.Lock()
	defer pwdMutex.Unlock()
	return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
}

func (p *PlayerInfo) SetPassword(pw string) {
	p.g.Lock()
	if len(p.p.Salt) < (256 / 8) {
		p.p.Salt = make([]byte, 256/8)
	}
	rand.Read(p.p.Salt)
	p.p.PWHash = "argon2"
	p.p.Password = pwdHash(p.p.Salt, pw)
	p.g.Unlock()
	p.g.changed <- struct{}{}
}

func (p *PlayerInfo) CheckPassword(pw string) bool {
	p.g.Lock()
	defer p.g.Unlock()
	switch p.p.PWHash {
	case "argon2":
		pwh := pwdHash(p.p.Salt, pw)
		return subtle.ConstantTimeCompare(p.p.Password, pwh) == 1
	default:
		// Unrecognized password hash
		return false
	}
}
