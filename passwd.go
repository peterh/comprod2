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
	game := state.Open(*data)
	if game == nil {
		fmt.Println("Unable to open game", *data)
		return
	}
	defer game.Close()
	var p *state.PlayerInfo
	errmsg := "No such user:"
	if flag.Arg(0) == "adduser" {
		p = game.NewPlayer(user)
		errmsg = "Cannot add user:"
	} else {
		p = game.Player(user)
	}
	if p == nil {
		fmt.Println(errmsg, user)
		return
	}
	p.SetPassword(password)
}
