package state

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/slices"
)

const sqliteDate = "2006-01-02 15:04:05"

func (g *Game) getPrevRun() (time.Time, error) {
	r := g.getGame.QueryRow("Time")
	prevString := ""
	r.Scan(&prevString)
	return time.Parse(sqliteDate, prevString)
}

func (g *Game) nextTurn() <-chan time.Time {
	now := time.Now().UTC()
	prev, err := g.getPrevRun()
	if err != nil || prev.Day() != now.Day() {
		return time.After(1)
	}

	tomorrow := now.Add(time.Hour * 23)
	for tomorrow.Day() == now.Day() {
		tomorrow = tomorrow.Add(time.Hour)
	}
	next := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
	return time.After(next.Sub(now))
}

func (g *Game) newDay() {
	const rounds = 15
	const (
		up = iota
		down
		dividend
	)

	now := time.Now().UTC()
	prev, err := g.getPrevRun()
	if err != nil && prev.Day() == now.Day() {
		// It's not quite tomorrow yet
		return
	}

	for {
		tx, _ := g.db.Begin()

		before := g.ListStocks()
		after := slices.Clone(before)

		var divpaid [stockTypes]uint64
		news := make([]string, 0, stockTypes)

		for i := 0; i < rounds; i++ {
			adjust := uint64(math.Pow(rand.Float64()*.8+1.2, 5.0))
			stock := rand.Intn(stockTypes)
			switch rand.Intn(3) {
			case up:
				after[stock].Value += adjust
				if after[stock].Value >= splitValue {
					news = append(news, after[stock].Name+" split 2 for 1")
					after[stock].Value = (after[stock].Value + 1) / 2
					before[stock].Value = (before[stock].Value + 1) / 2
					tx.Stmt(g.splitStock).Exec(stock + 1)
				}
			case down:
				if after[stock].Value <= adjust {
					news = append(news, after[stock].Name+" went bankrupt, and was removed from the market")
					tx.Stmt(g.bankruptStock).Exec(stock + 1)
					after[stock].Value = startingValue
					before[stock].Value = startingValue
					newname := g.pickName(tx)
					news = append(news, newname+" was added to the market")
					tx.Stmt(g.setStockName).Exec(stock+1, newname)
					after[stock].Name = newname
				} else {
					after[stock].Value -= adjust
				}
			case dividend:
				if after[stock].Value >= startingValue {
					divpaid[stock] += adjust
					tx.Stmt(g.dividendStock).Exec(stock+1, adjust)
				}
			}
		}

		for k, v := range after {
			var item string
			switch {
			case v.Value == before[k].Value:
				item = v.Name + " did not change price"
			case v.Value < before[k].Value:
				item = fmt.Sprintf("%s fell %.1f%%", v.Name, float64(before[k].Value-v.Value)/float64(before[k].Value)*100)
			default: // case v.Value > before[k].Value:
				item = fmt.Sprintf("%s rose %.1f%%", v.Name, float64(v.Value-before[k].Value)/float64(before[k].Value)*100)
			}
			if divpaid[k] > 0 {
				item = fmt.Sprintf("%s, and paid $%d in dividends", item, divpaid[k])
			}
			news = append(news, item)
			tx.Stmt(g.setStockValue).Exec(k+1, v.Value)
		}
		tx.Exec("DELETE FROM News")
		for _, n := range news {
			tx.Stmt(g.addNews).Exec(n)
		}

		if now.Month() != prev.Month() {
			leader := g.leaders(tx)
			if len(leader) > 0 {
				sort.Sort(LeaderSort(leader))
				announce := fmt.Sprintf("The winner of the %s %d season was %s, with a net worth of $%d",
					prev.Month().String(), prev.Year(), leader[0].Name, leader[0].Worth)
				tx.Stmt(g.addNews).Exec(announce)
				tx.Stmt(g.addHistory).Exec(announce)
			}
			if len(leader) > 1 {
				rup := []string{}
				for i := 1; i < len(leader); i++ {
					rup = append(rup, fmt.Sprintf("%s had $%d", leader[i].Name, leader[i].Worth))
				}
				tx.Stmt(g.addNews).Exec(fmt.Sprintf("(%s)", strings.Join(rup, ", ")))
			}
			g.reset(tx)
		}
		tx.Stmt(g.setGame).Exec("Time", now.Format(sqliteDate))
		err = tx.Commit()
		if err == nil {
			return
		}
		log.Println(err)
		tx.Rollback()
	}
}

func watcher(g *Game) {
	for {
		<-g.nextTurn()
		g.newDay()
	}
}
