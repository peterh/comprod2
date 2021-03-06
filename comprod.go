package main

import (
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/peterh/comprod2/state"
)

type handler struct {
	t   *template.Template
	err *template.Template
	g   *state.Game
}

type errorReason struct {
	Reason string
}

func login(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/login.html", 307)
}

func thinspForAgent(agent string) string {
	// IE before version 7 mishandles &thinsp;
	const IEtag = "MSIE "
	if i := strings.Index(agent, IEtag); i > 0 {
		version := agent[i+len(IEtag):]
		if dot := strings.Index(version, "."); dot > 0 {
			version = version[:dot]
			if ver, err := strconv.ParseUint(version, 10, 16); err == nil {
				if ver < 7 {
					return "&nbsp;"
				}
			}
		}
	}
	return "&thinsp;"
}

func formatValue(value uint64, sep string) template.HTML {
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
	return template.HTML(strings.Join(chunk, sep))
}

type formattedInfo struct {
	Name  string
	Worth template.HTML
}

func formatInfo(in []state.LeaderInfo, sep string) []formattedInfo {
	sort.Sort(state.LeaderSort(in))

	out := make([]formattedInfo, 0, len(in))
	for _, v := range in {
		out = append(out, formattedInfo{Name: v.Name, Worth: formatValue(v.Worth, sep)})
	}
	return out
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	token := r.FormValue("i")
	pw := r.FormValue("pw")
	var p *state.PlayerInfo

	if len(token) > 0 {
		// New user
		if len(name) < 1 || subtle.ConstantTimeCompare([]byte(token), []byte(inviteHash(h.g, name))) != 1 {
			h.err.Execute(w, &errorReason{"Invalid invitation"})
			return
		}
		p = h.g.NewPlayer(name)
		if p == nil {
			h.err.Execute(w, &errorReason{"You are already registered"})
			return
		}
		if len(pw) < 2 {
			h.err.Execute(w, &errorReason{"Please select a longer password"})
			return
		}
		p.SetPassword(pw)
		cookie := p.NewCookie()
		http.SetCookie(w, &http.Cookie{Name: "id", Value: base64.RawURLEncoding.EncodeToString(cookie)})
	} else if len(name) > 1 {
		// User login
		p = h.g.Player(name)
		// Don't do this in real code, by calculating the password hash after checking
		// for the presence of a user, an attacker can test for the presence of a user.
		if p == nil || !p.CheckPassword(pw) {
			h.err.Execute(w, &errorReason{"Invalid password or unknown user"})
			return
		}
		cookie := p.NewCookie()
		http.SetCookie(w, &http.Cookie{Name: "id", Value: base64.RawURLEncoding.EncodeToString(cookie)})
	} else {
		// Returning user
		c, err := r.Cookie("id")
		if err != nil {
			login(w, r)
			return
		}
		cookie, err := base64.RawURLEncoding.DecodeString(c.Value)
		if err == nil {
			name, p = h.g.PlayerByCookie(cookie)
		}
		if p == nil {
			login(w, r)
			return
		}
	}

	lotsstr := r.FormValue("lots")
	if len(lotsstr) > 0 {
		lots, err := strconv.ParseUint(lotsstr, 10, 64)
		if err != nil {
			h.err.Execute(w, &errorReason{err.Error()})
			return
		}
		action := r.FormValue("action")
		switch action {
		case "buy":
			err = p.Buy(r.FormValue("stock"), lots)
		case "sell":
			err = p.Sell(r.FormValue("stock"), lots)
		case "":
		default:
			h.err.Execute(w, &errorReason{"Unrecognized action: " + action})
			return
		}
		if err != nil {
			h.err.Execute(w, &errorReason{err.Error()})
			return
		}
	}
	thinsp := thinspForAgent(r.UserAgent()) // USA uses "," instead of "&thinsp;"

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
		News     []string
		Leader   []formattedInfo
		Invite   bool
	}
	s := h.g.ListStocks()
	d := &data{Name: name, News: h.g.News(), Leader: formatInfo(h.g.Leaders(), thinsp)}
	d.Invite = p.IsAdmin()
	ph := p.Holdings()
	nw := ph.Cash
	for k, v := range s {
		d.Stocks = append(d.Stocks, entry{
			Name:   v.Name,
			Cost:   v.Value,
			Shares: ph.Shares[k],
			Value:  formatValue(ph.Shares[k]*v.Value, thinsp),
		})
		nw += ph.Shares[k] * v.Value
	}
	d.Cash = formatValue(ph.Cash, thinsp)
	d.NetWorth = formatValue(nw, thinsp)
	h.t.Execute(w, d)
}

type inviter struct {
	t   *template.Template
	err *template.Template
	g   *state.Game
}

func (i *inviter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	token := r.FormValue("i")
	if len(name) < 1 || subtle.ConstantTimeCompare([]byte(token), []byte(inviteHash(i.g, name))) != 1 {
		i.err.Execute(w, &errorReason{"Invalid invitation"})
		return
	}
	if i.g.HasPlayer(name) {
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

func inviteUrl(game *state.Game, name string) string {
	return fmt.Sprintf("http://%s%s/invite?name=%s&i=%s", *hostname, *port, name, inviteHash(game, name))
}

type newer struct {
	t   *template.Template
	err *template.Template
	g   *state.Game
}

func (n *newer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("id")
	if err != nil {
		login(w, r)
		return
	}
	cookie, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		login(w, r)
		return
	}
	name, p := n.g.PlayerByCookie(cookie)
	if len(name) < 1 || p == nil {
		login(w, r)
		return
	}
	if !p.IsAdmin() {
		n.err.Execute(w, &errorReason{"Only the administrator can invite new players"})
		return
	}

	name = r.FormValue("invitee")
	if len(name) < 2 {
		n.err.Execute(w, &errorReason{"Please enter the name of the person you want to invite"})
		return
	}
	if n.g.HasPlayer(name) {
		n.err.Execute(w, &errorReason{name + " is already registered"})
		return
	}

	var d struct {
		Name, Invite string
	}
	d.Name = name
	d.Invite = inviteUrl(n.g, name)
	n.t.Execute(w, &d)
}

type adminer struct {
	t   *template.Template
	err *template.Template
	g   *state.Game
}

func (a *adminer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("id")
	if err != nil {
		login(w, r)
		return
	}
	cookie, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		login(w, r)
		return
	}
	name, p := a.g.PlayerByCookie(cookie)
	if len(name) < 1 || p == nil {
		login(w, r)
		return
	}
	if !p.IsAdmin() {
		a.err.Execute(w, &errorReason{"Only the administrator can access the admin console"})
		return
	}

	name = r.FormValue("delete")
	if len(name) >= 2 {
		var list = []struct {
			tag, human string
		}{
			{"sure", "sure"},
			{"rsure", "really sure"},
			{"vsure", "really very sure"},
			{"noundo", "understanding the gravity of the situation"},
		}
		for _, v := range list {
			if r.FormValue(v.tag) != "yes" {
				a.err.Execute(w, &errorReason{"You aren't " + v.human})
				return
			}
		}

		if !a.g.DeletePlayer(name) {
			a.err.Execute(w, &errorReason{name + " is not a registered player"})
			return
		}
	}

	var d struct {
		Players []state.LeaderInfo
	}
	d.Players = a.g.Leaders()
	a.t.Execute(w, &d)
}

type newpwer struct {
	t   *template.Template
	err *template.Template
	g   *state.Game
}

func (np *newpwer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("id")
	if err != nil {
		login(w, r)
		return
	}
	cookie, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		login(w, r)
		return
	}
	name, p := np.g.PlayerByCookie(cookie)
	if len(name) < 1 || p == nil {
		login(w, r)
		return
	}

	var d struct {
		Name    string
		Success bool
	}

	pw := r.FormValue("pw")
	if len(pw) > 1 {
		// Password Change
		old := r.FormValue("oldpw")
		if !p.CheckPassword(old) {
			np.err.Execute(w, &errorReason{"Invalid password"})
			return
		}
		pw2 := r.FormValue("pw2")
		if pw != pw2 {
			np.err.Execute(w, &errorReason{"New passwords do not match"})
			return
		}
		p.SetPassword(pw)
		d.Success = true
	}

	d.Name = name
	np.t.Execute(w, &d)
}

type historian struct {
	t *template.Template
	g *state.Game
}

func (h *historian) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var d struct {
		History []string
	}
	d.History = h.g.History()
	if len(d.History) < 1 {
		d.History = []string{"This game is too young to have a history"}
	}
	h.t.Execute(w, &d)
}

type logouter struct {
	g *state.Game
}

func (l *logouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("id"); err == nil {
		if cookie, err := base64.RawURLEncoding.DecodeString(c.Value); err == nil {
			if _, p := l.g.PlayerByCookie(cookie); p != nil {
				p.ClearCookie()
			}
		}
	}
	http.SetCookie(w, &http.Cookie{Name: "id", Value: ""})
	login(w, r)
}

//go:embed static templates
var fsbuiltin embed.FS

func start() {
	var fsroot fs.FS = fsbuiltin
	if *root != "" {
		fsroot = os.DirFS(*root)
	}

	gameTemplate, err := template.ParseFS(fsroot, path.Join("templates", "game.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	inviteTemplate, err := template.ParseFS(fsroot, path.Join("templates", "invite.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	newTemplate, err := template.ParseFS(fsroot, path.Join("templates", "new.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	historyTemplate, err := template.ParseFS(fsroot, path.Join("templates", "history.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	errorTemplate, err := template.ParseFS(fsroot, path.Join("templates", "error.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	adminTemplate, err := template.ParseFS(fsroot, path.Join("templates", "admin.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	newpwTemplate, err := template.ParseFS(fsroot, path.Join("templates", "newpw.html"))
	if err != nil {
		log.Fatal("Fatal Error: ", err)
	}

	staticfs, err := fs.Sub(fsroot, "static")
	if err != nil {
		log.Fatal("Fatal error opening static/: ", err)
	}
	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.FS(staticfs))))
	game := state.Open(*data)
	if game == nil {
		fmt.Println("Unable to open game", *data)
		return
	}
	game.Run()
	http.Handle("/", &handler{gameTemplate, errorTemplate, game})
	http.Handle("/invite", &inviter{inviteTemplate, errorTemplate, game})
	http.Handle("/newinvite", &newer{newTemplate, errorTemplate, game})
	http.Handle("/admin", &adminer{adminTemplate, errorTemplate, game})
	http.Handle("/newpw", &newpwer{newpwTemplate, errorTemplate, game})
	http.Handle("/history", &historian{historyTemplate, game})
	http.Handle("/logout", &logouter{game})

	log.Println("comprod started")

	log.Fatal(http.ListenAndServe(*port, nil))
}
