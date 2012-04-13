package main

import (
	"bytes"
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type handler struct {
	t   *template.Template
	err *template.Template
	g   *gameState
}

type errorReason struct {
	Reason string
}

var admin = flag.String("admin", "admin", "Name of administrator")

func login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/login.html", 307)
}

func formatValue(value uint64) template.HTML {
	s := strconv.FormatUint(value, 10)
	chunk := make([]string, 0)
	for len(s) > 0 {
		if len(s) >= 3 {
			chunk = append([]string{s[len(s)-3:]}, chunk...)
			s = s[:len(s)-3]
		} else {
			chunk = append([]string{s}, chunk...)
			s = ""
		}
	}
	return template.HTML(strings.Join(chunk, "&thinsp;"))
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	token := r.FormValue("i")
	pw := r.FormValue("pw")

	if len(token) > 0 {
		// New user
		if len(name) < 1 || token != inviteHash(name) {
			h.err.Execute(w, &errorReason{"Invalid invitation"})
			return
		}
		if h.g.hasPlayer(name) {
			h.err.Execute(w, &errorReason{"You are already registered"})
			return
		}
		if len(pw) < 2 {
			h.err.Execute(w, &errorReason{"Please select a longer password"})
			return
		}
		p := h.g.player(name)
		p.SetPassword(pwdHash(name, pw))
		http.SetCookie(w, &http.Cookie{Name: "id", Value: name + "/" + cookieHash(name)})
	} else if len(name) > 1 {
		// User login
		if !h.g.hasPlayer(name) {
			h.err.Execute(w, &errorReason{"Invalid password or unknown user"})
			return
		}
		p := h.g.player(name)
		if !bytes.Equal(p.Password, pwdHash(name, pw)) {
			h.err.Execute(w, &errorReason{"Invalid password or unknown user"})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "id", Value: name + "/" + cookieHash(name)})
	} else {
		// Returning user
		c, err := r.Cookie("id")
		if err != nil {
			login(w, r)
			return
		}
		i := strings.LastIndex(c.Value, "/")
		name = c.Value[:i]
		if len(name) < 1 || c.Value[i+1:] != cookieHash(name) {
			login(w, r)
			return
		}
	}

	type entry struct {
		Name   string
		Cost   uint64
		Shares uint64
		Value  template.HTML
	}
	type data struct {
		Name     string
		Stocks   []entry
		Cash     template.HTML
		NetWorth template.HTML
	}
	s := h.g.listStocks()
	p := h.g.player(name)
	d := &data{Name: name}
	nw := p.Cash
	for k, v := range s {
		d.Stocks = append(d.Stocks, entry{
			Name:   v.Name,
			Cost:   v.Value,
			Shares: p.Shares[k],
			Value:  formatValue(p.Shares[k] * v.Value),
		})
		nw += p.Shares[k] * v.Value
	}
	d.Cash = formatValue(p.Cash)
	d.NetWorth = formatValue(nw)
	h.t.Execute(w, d)
}

type inviter struct {
	t   *template.Template
	err *template.Template
	g   *gameState
}

func (i *inviter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	token := r.FormValue("i")
	if len(name) < 1 || token != inviteHash(name) {
		i.err.Execute(w, &errorReason{"Invalid invitation"})
		return
	}
	if i.g.hasPlayer(name) {
		i.err.Execute(w, &errorReason{"You are already registered"})
		return
	}

	var d struct {
		Name, Invite string
	}
	d.Name = name
	d.Invite = token
	i.t.Execute(w, &d)
}

func main() {
	flag.Parse()

	gameTemplate, err := template.ParseFiles(filepath.Join(*root, "templates", "game.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	inviteTemplate, err := template.ParseFiles(filepath.Join(*root, "templates", "invite.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	errorTemplate, err := template.ParseFiles(filepath.Join(*root, "templates", "error.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.Dir(filepath.Join(*root, "static")))))
	game := newGame()
	http.Handle("/", &handler{gameTemplate, errorTemplate, game})
	http.Handle("/invite", &inviter{inviteTemplate, errorTemplate, game})

	log.Println("comprod started")
	log.Printf("To start, visit http://%s%s/invite?name=%s&i=%s\n", *hostname, *port, *admin, inviteHash(*admin))

	log.Fatal(http.ListenAndServe(*port, nil))
}
