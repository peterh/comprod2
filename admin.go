package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/peterh/comprod2/state"
)

func admin() {
	name := flag.Arg(1)
	is := flag.Arg(2)
	if len(name) < 1 || len(is) < 1 {
		flag.Usage()
		return
	}
	game := state.Open(*data)
	if game == nil {
		fmt.Println("Unable to open game", *data)
		return
	}
	defer game.Close()
	setto := false
	switch strings.ToLower(is) {
	case "true", "1", "on", "yes":
		setto = true
	}
	p := game.Player(name)
	if p == nil {
		fmt.Println("No such user:", name)
		return
	}
	p.SetAdmin(setto)
}
