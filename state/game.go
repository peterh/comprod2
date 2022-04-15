package state

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"math/rand"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const stockTypes = 6
const startingValue = 100
const splitValue = startingValue * 2
const startingCash = 100000

//go:embed sql/create
var sqlCreate string

//go:embed sql/getgame
var getGame string

//go:embed sql/setgame
var setGame string

//go:embed sql/setpassword
var setPassword string

//go:embed sql/getpassword
var getPassword string

//go:embed sql/findstock
var findStock string

//go:embed sql/findplayer
var findPlayer string

//go:embed sql/findbycookie
var findPlayerByCookie string

//go:embed sql/setcookie
var setCookie string

//go:embed sql/addplayer
var addPlayer string

//go:embed sql/deleteplayer
var deletePlayer string

//go:embed sql/addstock
var addStock string

//go:embed sql/buy
var buyStock string

//go:embed sql/sell
var sellStock string

//go:embed sql/liststocks
var listStocks string

//go:embed sql/setstockname
var setStockName string

//go:embed sql/setstockvalue
var setStockValue string

//go:embed sql/split
var splitStock string

//go:embed sql/bankrupt
var bankruptStock string

//go:embed sql/dividend
var dividendStock string

//go:embed sql/addnews
var addNews string

//go:embed sql/getnews
var getNews string

//go:embed sql/addhistory
var addHistory string

//go:embed sql/gethistory
var getHistory string

//go:embed sql/getholding
var getHolding string

//go:embed sql/setholding
var setHolding string

//go:embed sql/getadmin
var getAdmin string

//go:embed sql/setadmin
var setAdmin string

//go:embed sql/getleaders
var getLeaders string

//go:embed sql/reset
var resetGame string

type PlayerHoldings struct {
	Cash   uint64
	Shares [stockTypes]uint64
}

type Stock struct {
	Name  string
	Value uint64
}

type Game struct {
	db                          *sql.DB
	getGame, setGame            *sql.Stmt
	getPassword, setPassword    *sql.Stmt
	findStockIndex              *sql.Stmt
	addPlayer                   *sql.Stmt
	findPlayer, deletePlayer    *sql.Stmt
	findPlayerByCookie          *sql.Stmt
	setCookie                   *sql.Stmt
	getHolding, getLeaders      *sql.Stmt
	setHolding                  *sql.Stmt
	addStock                    *sql.Stmt
	setStockName, setStockValue *sql.Stmt
	splitStock, bankruptStock   *sql.Stmt
	dividendStock               *sql.Stmt
	buy, sell                   *sql.Stmt
	listStocks                  *sql.Stmt
	getAdmin, setAdmin          *sql.Stmt
	getNews, addNews            *sql.Stmt
	getHistory, addHistory      *sql.Stmt
	resetGame                   *sql.Stmt
}

type PlayerInfo struct {
	playerID int
	g        *Game
}

type LeaderInfo struct {
	Name  string
	Worth uint64
}

func (g *Game) findStock(tx *sql.Tx, stock string) int {
	index := -1
	r := tx.Stmt(g.findStockIndex).QueryRow(stock)
	r.Scan(&index)
	return index
}

func isBusy(err error) bool {
	serr, ok := err.(*sqlite.Error)
	if !ok {
		return false
	}
	return serr.Code() == sqlite3.SQLITE_BUSY
}

func (p *PlayerInfo) Buy(stock string, lots uint64) error {
	var tx *sql.Tx
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	shares := lots * 100
	for {
		tx, _ = p.g.db.Begin()
		idx := p.g.findStock(tx, stock)
		if idx < 0 {
			return fmt.Errorf("%s is not on the market", stock)
		}

		r := tx.Stmt(p.g.buy).QueryRow(p.playerID, idx, shares)
		var cash int64
		err := r.Scan(&cash)
		if err != nil {
			if isBusy(err) {
				tx.Rollback()
				continue
			}
			return err
		}
		if cash < 0 {
			return fmt.Errorf("You don't have enough cash to buy %d shares of %s", lots*100, stock)
		}
		err = tx.Commit()
		if isBusy(err) {
			tx.Rollback()
			continue
		}
		if err == nil {
			tx.Commit()
		}
		return err
	}
}

func (p *PlayerInfo) Sell(stock string, lots uint64) error {
	var tx *sql.Tx
	defer func() {
		if tx != nil {
			tx.Rollback()
		}
	}()
	shares := lots * 100
	for {
		tx, _ = p.g.db.Begin()
		idx := p.g.findStock(tx, stock)
		if idx < 0 {
			return fmt.Errorf("%s is not on the market", stock)
		}

		r := tx.Stmt(p.g.sell).QueryRow(p.playerID, idx, shares)
		sharesRemain := int64(-1)
		err := r.Scan(&sharesRemain)
		if err != nil && err != sql.ErrNoRows {
			if isBusy(err) {
				tx.Rollback()
				continue
			}
			return err
		}
		if sharesRemain < 0 {
			return fmt.Errorf("You don't have %d shares of %s to sell", shares, stock)
		}
		err = tx.Commit()
		if isBusy(err) {
			tx.Rollback()
			continue
		}
		if err == nil {
			tx.Commit()
		}
		return err
	}
}

func (p *PlayerInfo) Holdings() PlayerHoldings {
	var rv PlayerHoldings
	r := p.g.getHolding.QueryRow(p.playerID, "Cash")
	r.Scan(&rv.Cash)
	for i := 1; i <= stockTypes; i++ {
		r = p.g.getHolding.QueryRow(p.playerID, i)
		r.Scan(&rv.Shares[i-1])
	}
	return rv
}

func (p *PlayerInfo) IsAdmin() bool {
	r := p.g.getAdmin.QueryRow(p.playerID)
	rv := false
	err := r.Scan(&rv)
	if err != nil {
		return false
	}
	return rv
}

func (p *PlayerInfo) SetAdmin(is bool) {
	p.g.setAdmin.Exec(p.playerID, is)
}

func (g *Game) ListStocks() []Stock {
	rv := make([]Stock, 0)
	r, err := g.listStocks.Query()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	for r.Next() {
		var s Stock
		r.Scan(&s.Name, &s.Value)
		rv = append(rv, s)
	}
	return rv
}

func getStrings(s *sql.Stmt) []string {
	rv := make([]string, 0)
	r, err := s.Query()
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	for r.Next() {
		var s string
		r.Scan(&s)
		rv = append(rv, s)
	}
	return rv
}

func (g *Game) History() []string {
	return getStrings(g.getHistory)
}

func (g *Game) HasPlayer(name string) bool {
	return g.Player(name) != nil
}

func (g *Game) DeletePlayer(name string) bool {
	tx, err := g.db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	idx := -1
	r := tx.Stmt(g.findPlayer).QueryRow(name)
	err = r.Scan(&idx)
	if err != nil {
		return false
	}
	_, err = tx.Stmt(g.deletePlayer).Exec(idx)
	if err != nil {
		return false
	}
	tx.Commit()
	return true
}

func (g *Game) Player(name string) *PlayerInfo {
	rv := PlayerInfo{g: g}
	r := g.findPlayer.QueryRow(name)
	err := r.Scan(&rv.playerID)
	if err != nil {
		return nil
	}
	return &rv
}

func (g *Game) PlayerByCookie(cookie []byte) (string, *PlayerInfo) {
	if cookie == nil || len(cookie) < 10 {
		return "", nil
	}
	rv := PlayerInfo{g: g}
	var name string
	r := g.findPlayerByCookie.QueryRow(cookie)
	err := r.Scan(&name, &rv.playerID)
	if err != nil {
		return "", nil
	}
	return name, &rv
}

func (g *Game) NewPlayer(name string) *PlayerInfo {
	rv := PlayerInfo{g: g, playerID: -1}
	for {
		tx, _ := g.db.Begin()
		r := tx.Stmt(g.addPlayer).QueryRow(name)
		err := r.Scan(&rv.playerID)
		if err != nil && isBusy(err) {
			tx.Rollback()
			continue
		}
		if err != nil || rv.playerID < 0 {
			tx.Rollback()
			return nil
		}
		tx.Stmt(g.setHolding).Exec(rv.playerID, "Cash", startingCash)
		if tx.Commit() == nil {
			return &rv
		}
		tx.Rollback()
	}
}

func (g *Game) Leaders() []LeaderInfo {
	var rv []LeaderInfo
	r, err := g.getLeaders.Query()
	if err != nil {
		return rv
	}
	defer r.Close()
	for r.Next() {
		var li LeaderInfo
		r.Scan(&li.Name, &li.Worth)
		rv = append(rv, li)
	}
	return rv
}

func (g *Game) News() []string {
	return getStrings(g.getNews)
}

func (g *Game) pickName(t *sql.Tx) string {
	names := [...]string{"Coffee", "Soybeans", "Corn", "Wheat", "Cocoa", "Gold", "Silver", "Platinum", "Oil", "Natural Gas", "Cotton", "Sugar"}
	used := make(map[string]bool)
	r, err := t.Stmt(g.listStocks).Query()
	if err == nil {
		defer r.Close()
		for r.Next() {
			var name string
			var value int
			r.Scan(&name, &value)
			used[name] = true
		}
	}
	for {
		i := rand.Intn(len(names))
		if !used[names[i]] {
			return names[i]
		}
	}
}

func (g *Game) reset(t *sql.Tx) {
	s := g.resetGame
	if t != nil {
		s = t.Stmt(s)
	}
	s.Exec(startingCash)
	s = g.addStock
	if t != nil {
		s = t.Stmt(s)
	}
	for i := 1; i <= stockTypes; i++ {
		s.Exec(i, g.pickName(t), startingValue)
	}
}

func mustPrepare(db *sql.DB, stmt string) *sql.Stmt {
	s, err := db.Prepare(stmt)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func (g *Game) getKey() []byte {
	r := g.getGame.QueryRow("Key")
	var rv []byte
	err := r.Scan(&rv)
	if err != nil {
		log.Fatal(err)
	}
	return rv
}

func (g *Game) prepareAll() {
	db := g.db
	g.getGame = mustPrepare(db, getGame)
	g.setGame = mustPrepare(db, setGame)
	g.setPassword = mustPrepare(db, setPassword)
	g.getPassword = mustPrepare(db, getPassword)
	g.findStockIndex = mustPrepare(db, findStock)
	g.addPlayer = mustPrepare(db, addPlayer)
	g.findPlayer = mustPrepare(db, findPlayer)
	g.findPlayerByCookie = mustPrepare(db, findPlayerByCookie)
	g.setCookie = mustPrepare(db, setCookie)
	g.deletePlayer = mustPrepare(db, deletePlayer)
	g.addStock = mustPrepare(db, addStock)
	g.buy = mustPrepare(db, buyStock)
	g.sell = mustPrepare(db, sellStock)
	g.listStocks = mustPrepare(db, listStocks)
	g.setStockName = mustPrepare(db, setStockName)
	g.setStockValue = mustPrepare(db, setStockValue)
	g.splitStock = mustPrepare(db, splitStock)
	g.bankruptStock = mustPrepare(db, bankruptStock)
	g.dividendStock = mustPrepare(db, dividendStock)
	g.addNews = mustPrepare(db, addNews)
	g.getNews = mustPrepare(db, getNews)
	g.addHistory = mustPrepare(db, addHistory)
	g.getHistory = mustPrepare(db, getHistory)
	g.getHolding = mustPrepare(db, getHolding)
	g.setHolding = mustPrepare(db, setHolding)
	g.getLeaders = mustPrepare(db, getLeaders)
	g.getAdmin = mustPrepare(db, getAdmin)
	g.setAdmin = mustPrepare(db, setAdmin)
	g.resetGame = mustPrepare(db, resetGame)
}

func Open(data string) *Game {
	rand.Seed(GetSeed())

	var g Game

	db, err := sql.Open("sqlite", data)
	if err != nil {
		log.Fatal(err)
	}
	g.db = db

	// check contents of db here
	row := db.QueryRow(string(getGame), "Key")
	var key []byte
	err = row.Scan(&key)
	if err != nil || len(key) < 10 {
		db.Close()
		return nil
	}
	g.prepareAll()

	return &g
}

func Create(data string) *Game {
	rand.Seed(GetSeed())

	var g Game

	db, err := sql.Open("sqlite", data)
	if err != nil {
		log.Fatal(err)
	}
	g.db = db

	// check contents of db here
	row := db.QueryRow(string(getGame), "Key")
	var key []byte
	err = row.Scan(&key)
	if err == nil && len(key) >= 10 {
		db.Close()
		return nil
	}
	_, err = db.Exec(sqlCreate)
	if err != nil {
		log.Fatal(err)
	}

	g.prepareAll()
	g.setGame.Exec("Key", newKey())
	g.reset(nil)

	return &g
}

func (g *Game) Run() {
	go watcher(g)
}

func (g *Game) Close() {
	g.db.Close()
}
