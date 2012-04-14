package state

import (
	"encoding/gob"
	"log"
	"os"
	"time"
)

func (g *Game) write(fn string) {
	g.Lock()
	defer g.Unlock()

	new := fn + ".new"
	old := fn + ".old"
	f, err := os.Create(fn + ".new")
	if err != nil {
		log.Fatal(err)
	}
	err = gob.NewEncoder(f).Encode(&g.g)
	if err != nil {
		log.Fatal(err)
	}
	f.Sync()
	f.Close()

	os.Rename(fn, old)
	err = os.Rename(new, fn)
	if err != nil {
		log.Fatal(err)
	}
	os.Remove(old)
}

func watcher(g *Game, filename string, changed chan struct{}) {
	var tick <-chan time.Time
	for {
		select {
		case <-changed:
			if tick == nil {
				tick = time.After(5 * time.Minute)
			}
		case <-tick:
			tick = nil
			g.write(filename)
		}
	}
}
