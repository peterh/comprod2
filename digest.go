package main

import (
	"encoding/base64"
	"strings"

	"github.com/peterh/comprod2/state"
)

func doHash(g *state.Game, thing, name string) string {
	sum := g.Hash(thing, name)
	return strings.TrimRight(base64.URLEncoding.EncodeToString(sum), "=")
}

func cookieHash(g *state.Game, name string) string {
	return doHash(g, "cookie", name)
}

func inviteHash(g *state.Game, name string) string {
	return doHash(g, "invite", name)
}
