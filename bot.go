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

	var drawings MultiDrawer
	for _, zone := range config.Zones {
		var drawing Drawer

		var fill FillGenerator
		switch zone.Fill {
		case "spiral", "":
			fill = &SpiralFill{}
		case "random":
			fill = &RandomFill{}

		default:
			log.Fatalf("invalid fill type: %s", zone.Fill)

		}

		switch zone.Draw {
		case "bitmap", "":
			drawing = NewBitmapDraw(zone.Position, fill, zone.Data)
		default:
			log.Fatalf("invalid zone type: %s", zone.Draw)
		}

		drawings = append(drawings, drawing)
	}

	go updateBitmap()

	// wait for bitmap
	for Bitmap == nil {
		time.Sleep(time.Second)
	}

	for _, user := range config.Users {
		go userHandler(user, drawings)
		time.Sleep(time.Second)
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
	Skip     bool   `yaml:"skip"`
	Position []int  `yaml:"position,flow"`
	Draw     string `yaml:"draw"`
	Fill     string `yaml:"fill"`
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

	timer := time.Tick(3 * time.Minute)
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
		log.Fatalf("%s: could not log in: %v", user.User, err)
	}

	log.Printf("%s logged in", user.User)

	wait, err := c.WaitTime()
	if err != nil {
		log.Fatal(err)
	}

	for {
		time.Sleep(wait)
		drawSync.Lock()

		t := drawer.Next()
		if t == nil {
			wait = 1 * time.Second
			drawSync.Unlock()
			continue
		}

		wait, err = c.Draw(*t)
		if err != nil {
			log.Printf("user %s failed to draw: %s", user.User, err)
			drawSync.Unlock()
			return
		}

		Bitmap[t.Y][t.X] = t.Color

		drawSync.Unlock()
		log.Printf("%s has drawn %d at (%d, %d), waiting %v", user.User, t.Color, t.X, t.Y, wait)
	}
}
