package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	loginURL = "https://www.reddit.com/api/login/"
	meURL    = "https://www.reddit.com/api/me.json"

	infoURL   = "https://www.reddit.com/api/place/pixel.json"
	bitmapURL = "https://www.reddit.com/api/place/board-bitmap"
	timeURL   = "https://www.reddit.com/api/place/time.json"
	drawURL   = "https://www.reddit.com/api/place/draw.json"
)

var wsURLRe = regexp.MustCompile(`"wss://.*?"`)

type Tile struct {
	X     int  `json:"x"`
	Y     int  `json:"y"`
	Color byte `json:"color"`
}

type Info struct {
	Username  string  `json:"user_name"`
	Timestamp float64 `json:"timestamp"`

	Tile
}

func GetBitmap() ([][]byte, error) {
	res, err := http.Get(bitmapURL)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	defer res.Body.Close()

	r := bufio.NewReader(res.Body)
	var whatIsThis [4]byte
	r.Read(whatIsThis[:])

	buf := make([]byte, 1000000, 1000000)
	for i := 0; i < len(buf); i += 2 {
		pix, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		buf[i] = pix >> 4
		buf[i+1] = pix & 15
	}

	bitmap := make([][]byte, 1000)
	for i := range bitmap {
		bitmap[i] = buf[i*1000 : (i+1)*1000]
	}

	return bitmap, nil
}

func GetPixel(x, y int) (*Info, error) {
	query := fmt.Sprintf("?x=%d&y=%d", x, y)

	res, err := http.Get(infoURL + query)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	defer res.Body.Close()

	var inf Info
	if err := json.NewDecoder(res.Body).Decode(&inf); err != nil {
		return nil, err
	}

	return &inf, nil
}

type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type TileUpdate struct {
	Author string `json:"author"`

	Tile
}

func getWSURL() (string, error) {
	req, err := http.NewRequest("GET", "https://www.reddit.com/place?webview=true", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36 OPR/43.0.2442.1144")

	res, err := http.DefaultClient.Do(req)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	wsURL := wsURLRe.FindString(string(body))
	if wsURL == "" {
		return "", errors.New("could not find websocket url")
	}

	return strings.Trim(wsURL, "\""), nil
}

func ReadChanges(c *websocket.Conn) <-chan TileUpdate {
	ch := make(chan TileUpdate)

	go func(c *websocket.Conn, ch chan<- TileUpdate) {
		defer close(ch)
		for {
			var evt Event
			if err := c.ReadJSON(&evt); err != nil {
				log.Fatal(err)
				return
			}

			switch evt.Type {
			case "place":
				var tile TileUpdate
				if err := json.Unmarshal([]byte(evt.Payload), &tile); err != nil {
					log.Fatal(err)
					return
				}
				ch <- tile

			case "batch-place":
				var tiles []TileUpdate
				if err := json.Unmarshal([]byte(evt.Payload), &tiles); err != nil {
					log.Fatal(err)
					return
				}
				for _, t := range tiles {
					ch <- t
				}

			case "activity":
				log.Printf("%s\n", evt.Payload)
			default:
				log.Println("unknown event with type:", evt.Type)
			}
		}
	}(c, ch)

	return ch
}

type Client struct {
	c *http.Client
}

func NewClient(u User) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	headers := headerRoundTripper{
		"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36 OPR/43.0.2442.1144",
	}

	client := &http.Client{
		Jar:       jar,
		Transport: headers,
	}

	res, err := client.PostForm(loginURL+u.User, url.Values{
		"user":     {u.User},
		"passwd":   {u.Pass},
		"api_type": {"json"},
	})
	if err != nil {
		return nil, err
	}

	res, err = client.Get(meURL)
	if err != nil {
		return nil, err
	}

	var mhr struct {
		Data struct {
			ModHash string `json:"modhash"`
		} `json:"data"`
	}
	err = decodeJSON(res, &mhr)
	if err != nil {
		return nil, err
	}

	if mhr.Data.ModHash == "" {
		return nil, errors.New("could not login")
	}

	headers["x-modhash"] = mhr.Data.ModHash

	return &Client{
		c: client,
	}, nil
}

func (c *Client) WaitTime() (time.Duration, error) {
	res, err := c.c.Get(timeURL)
	if err != nil {
		return 0, err
	}

	var tr struct {
		WaitSeconds float64 `json:"wait_seconds"`
	}
	err = decodeJSON(res, &tr)
	if err != nil {
		return 0, err
	}

	return time.Duration(math.Floor(tr.WaitSeconds+.99)) * time.Second, nil
}

func (c *Client) Draw(t Tile) (time.Duration, error) {
	res, err := c.c.PostForm(drawURL, url.Values{
		"x":     {strconv.Itoa(t.X)},
		"y":     {strconv.Itoa(t.Y)},
		"color": {strconv.Itoa(int(t.Color))},
	})

	var tr struct {
		WaitSeconds float64 `json:"wait_seconds"`
	}
	err = decodeJSON(res, &tr)
	if err != nil {
		return 0, err
	}

	return time.Duration(math.Floor(tr.WaitSeconds+.99)) * time.Second, nil
}

func postJSON(c *http.Client, url string, v interface{}) (*http.Response, error) {
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(v)
	if err != nil {
		return nil, err
	}

	res, err := c.Post(url, "application/json", &buf)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func decodeJSON(res *http.Response, v interface{}) error {
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(v)
}

type headerRoundTripper map[string]string

func (rt headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range rt {
		req.Header.Add(k, v)
	}

	return http.DefaultTransport.RoundTrip(req)
}
