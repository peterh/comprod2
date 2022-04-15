package main

import (
	"flag"
	"fmt"

	"github.com/peterh/comprod2/state"
)

func passwd() {
	user := flag.Arg(1)
	password := flag.Arg(2)
	if len(user) < 1 || len(password) < 1 {
		flag.Usage()
		return
	}
	game := state.New(*data)
	defer game.Close()
	p := game.Player(user)
	if p == nil {
		fmt.Println("No such user:", user)
		return
	}
	p.SetPassword(password)
}
