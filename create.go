package main

import (
	"fmt"

	"github.com/peterh/comprod2/state"
)

func create() {
	g := state.Create(*data)
	if g == nil {
		fmt.Println("Unable to create", *data)
		return
	}
	g.Close()
}
