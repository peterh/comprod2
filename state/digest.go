package state

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"hash"
	"io"
	"log"
	"sync"

	"golang.org/x/crypto/argon2"
)

// newKey must be called under the lock (or in a context
// where the lock is unnecessary)
func (g *GameState) newKey() {
	g.Key = make([]byte, sha1.Size)
	_, err := io.ReadFull(rand.Reader, g.Key)
	if err != nil {
		log.Fatal(err)
	}
}

func (g *Game) GetHash() hash.Hash {
	g.Lock()
	defer g.Unlock()
	return hmac.New(sha1.New, g.g.Key)
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
