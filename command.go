package main

import (
	"flag"
	"fmt"
)

var command = []struct {
	f    func()
	name string
	desc string
}{
	{f: passwd, name: "adduser", desc: "<user> <password> Add a new user"},
	{f: create, name: "create", desc: "Create new empty game"},
	{f: invite, name: "invite", desc: "<user> Invite a new user to the game"},
	{f: passwd, name: "passwd", desc: "<user> <password> Set a user's password"},
	{f: start, name: "start", desc: "Start a web server to run the game"},
}

func usage() {
	fmt.Println("Usage: comprod2 [flags] <command>")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Commands:")
	for _, v := range command {
		fmt.Printf("  %8s %s\n", v.name, v.desc)
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()
	cmd := flag.Arg(0)
	for _, v := range command {
		if v.name == cmd {
			v.f()
			return
		}
	}
	flag.Usage()
}
