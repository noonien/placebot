package main

import (
	"flag"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	yaml "gopkg.in/yaml.v2"
)

var (
	configFile = flag.String("conf", "config.yaml", "configuration file")
)

func main() {
	flag.Parse()

	config := &Config{}
	cb, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal(cb, config)
	if err != nil {
		log.Fatal(err)
	}

	zones := MultiDrawer{}
	for _, zone := range config.Zones {
		switch zone.Type {
		case "spiraldraw":
			zones = append(zones, NewSpiralDraw(zone.Position, zone.Data))
		default:
			log.Fatal("invalid zone type: %s", zone.Type)
		}
	}

	go updateBitmap()

	for _, user := range config.Users {
		go userHandler(user, zones)
	}

	select {}
}

type Config struct {
	Users []User           `yaml:"users"`
	Zones map[string]*Zone `yaml:"zones"`
}

type User struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
}

type Zone struct {
	Position []int  `yaml:"position,flow"`
	Type     string `yaml:"type"`
	Data     string `yaml:"data"`
}

var Bitmap [][]byte

func updateBitmap() {
	wsURL, err := getWSURL()
	if err != nil {
		log.Fatal(err)
	}

	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatal(err)
	}

	Bitmap, err = GetBitmap()
	if err != nil {
		log.Fatal(err)
	}

	timer := time.Tick(5 * time.Minute)
	changes := ReadChanges(c)

	for {
		select {
		case <-timer:
			Bitmap, err = GetBitmap()
			if err != nil {
				log.Fatal(err)
			}

		case tile := <-changes:
			if 0 < tile.X && tile.X < 1000 && 0 < tile.Y && tile.Y < 1000 {
				Bitmap[tile.Y][tile.X] = tile.Color
			}

		}
	}
}

var drawSync sync.Mutex

func userHandler(user User, drawer Drawer) {
	c, err := NewClient(user)
	if err != nil {
		log.Fatal(user.User, err)
	}

	// wait for bitmap
	for Bitmap == nil {
		time.Sleep(time.Second)
	}

	wait, err := c.WaitTime()
	if err != nil {
		log.Fatal(err)
	}

	if wait > 0 {
		log.Printf("%s waiting %v", user.User, wait)
	}

	for {
		time.Sleep(wait)
		drawSync.Lock()

		t := drawer.Next()
		if t == nil {
			wait = time.Second
			continue
		}

		Bitmap[t.Y][t.X] = t.Color

		wait, err = c.Draw(*t)
		if err != nil {
			log.Fatal(err)
		}

		drawSync.Unlock()
		log.Printf("%s has drawn %d at (%d, %d), waiting %v", user.User, t.Color, t.X, t.Y, wait)
	}
}
