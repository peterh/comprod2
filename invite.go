package main

import (
	"flag"
	"fmt"

	"github.com/peterh/comprod2/state"
)

func invite() {
	name := flag.Arg(1)
	if len(name) < 1 {
		flag.Usage()
		return
	}
	game := state.Open(*data)
	if game == nil {
		fmt.Println("Unable to open game", *data)
		return
	}
	if game.HasPlayer(name) {
		fmt.Println(name, "is already part of the game")
		return
	}
	fmt.Printf("To join the game as %s, visit %s\n", name, inviteUrl(game, name))
}
