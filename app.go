package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/robfig/cron"
	"github.com/taq-f/garoonchecker/server"
	exists "github.com/taq-f/go-exists"
)

var config = new(Config)
var historyFile string
var jars = map[string]http.CookieJar{"api": nil, "web": nil}
var ticket = map[string]string{}

func main() {
	fmt.Println("start main")
	readConfig()
	if config.Debug {
		fmt.Println("starting debug server")
		go server.Start()
	}
	connect()

	c := cron.New()
	c.AddFunc("*/5 * * * * *", func() {
		updates := getUpdates()
		filtered := filterByHistory(updates)
		fmt.Println("notify", filtered)
	})
	go c.Start()

	// prevent this app quit
	for {
		time.Sleep(10000000000000)
	}
}

func init() {
	// create work directory
	dir, err := homedir.Dir()

	if err != nil {
		fmt.Println("counld not detect home directory.")
		return
	}

	var workDirPath = path.Join(dir, ".garoonchecker")

	if !exists.File(workDirPath) {
		err = os.Mkdir(workDirPath, 0777)
		if err != nil {
			fmt.Println("failed to create app work directory", err)
		}

	}

	historyFile = path.Join(workDirPath, "history.json")

	if !exists.File(historyFile) {
		file, err := os.Create(historyFile)

		if err != nil {
			fmt.Println("error on opening history file")
		}
		defer file.Close()

		inititalHistory := new(History)
		inititalHistory.Ids = []int{}
		toWrite, _ := json.Marshal(inititalHistory)

		file.Write(toWrite)
	}

	// Cookie jar
	jars["api"], _ = cookiejar.New(nil)
	jars["web"], _ = cookiejar.New(nil)
}

func readConfig() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}

	var configPath string
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	} else {
		configPath = path.Join(path.Dir(ex), "config.json")
	}

	raw, err := ioutil.ReadFile(configPath)
	fmt.Println(configPath, string(raw))
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	json.Unmarshal(raw, config)
}

func connect() {
	username := config.Garoon.Account.Username
	password := config.Garoon.Account.Password

	retApi := loginApi(username, password)
	retWeb := loginWeb(username, password)

	if !(retApi && retWeb) {
		fmt.Println("failed to login")
		os.Exit(1)
	}

	ticket = getTicket()

	if ticket == nil {
		fmt.Println("failed to get ticket")
		return
	}
}

func loginApi(username string, password string) bool {
	data := LoginInfo{username, password}
	b, err := json.Marshal(data)
	if err != nil {
		return false
	}

	client := &http.Client{
		Jar: jars["api"],
	}

	resp, _ := client.Post(
		config.Garoon.Url.LoginApi,
		"application/json",
		bytes.NewBuffer(b),
	)
	defer resp.Body.Close()
	byteArray, err := ioutil.ReadAll(resp.Body)
	result := new(LoginResult)

	if err := json.Unmarshal(byteArray, result); err != nil {
		fmt.Println("JSON Unmarshal error:", err)
		return false
	}

	return result.Success
}

func loginWeb(username string, password string) bool {
	client := &http.Client{
		Jar: jars["web"],
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Add("_account", username)
	data.Add("_password", password)

	resp, _ := client.Post(
		config.Garoon.Url.LoginWeb,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	defer resp.Body.Close()

	return resp.StatusCode == 302
}

func getTicket() map[string]string {
	client := &http.Client{
		Jar: jars["web"],
	}

	resp, _ := client.Get(
		config.Garoon.Url.Portal,
	)
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(resp)

	if err != nil {
		fmt.Println("error on new doc")
		return nil
	}

	t := map[string]string{}

	doc.Find("form[name^=mail_receive] input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		// fmt.Println(s)
		val, _ := s.Attr("value")
		name, _ := s.Attr("name")
		t[name] = val
	})

	return t
}

func receiveMail(t map[string]string) bool {
	client := &http.Client{
		Jar: jars["web"],
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Add("csrf_ticket", t["csrf_ticket"])
	data.Add("aid", "264")
	data.Add("cmd", t["cmd"])

	resp, _ := client.Post(
		config.Garoon.Url.ReceiveEmail,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	defer resp.Body.Close()

	return resp.StatusCode == 302
}

func getUpdates() *Updates {
	receiveMail(ticket)

	client := &http.Client{
		Jar: jars["api"],
	}

	t := time.Now().UTC()
	const l = "2006-01-02T15:04:05Z"

	ed := t.Add(-24 * time.Hour)
	st := t

	data := ReqeustUpdatesInfo{Start: st.Format(l), End: ed.Format(l)}
	b, _ := json.Marshal(data)

	resp, _ := client.Post(
		config.Garoon.Url.NotificationList,
		"application/json",
		bytes.NewBuffer(b),
	)
	byteArray, _ := ioutil.ReadAll(resp.Body)
	result := new(Updates)

	if err := json.Unmarshal(byteArray, result); err != nil {
		fmt.Println("JSON Unmarshal error:", err)
		return nil
	}
	defer resp.Body.Close()

	return result
}

func filterByHistory(updates *Updates) []int {
	// read history
	raw, err := ioutil.ReadFile(historyFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// var history []string
	history := new(History)
	json.Unmarshal(raw, history)

	filtered := []int{}

	for i := 0; i < len(updates.Mail); i++ {
		// it the upadate in history array?
		u := updates.Mail[i]

		contains := false

		for j := 0; j < len(history.Ids); j++ {
			h := history.Ids[j]
			if u.Id == h {
				contains = true
				break
			}
		}
		if !contains {
			filtered = append(filtered, u.Id)
		}
	}

	// save new updates to the history file
	summedUp := append(history.Ids, filtered...)

	newHistory := new(History)
	newHistory.Ids = summedUp

	s, err := json.Marshal(newHistory)
	if err != nil {
		fmt.Println("error!")
		return nil
	}

	ioutil.WriteFile(historyFile, s, os.ModePerm)

	return filtered
}

type Config struct {
	Debug  bool   `json:"debug"`
	Garoon Garoon `json:"garoon"`
}

type Garoon struct {
	Account struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"account"`
	Url struct {
		LoginWeb         string `json:"loginWeb"`
		LoginApi         string `json:"loginApi"`
		ReceiveEmail     string `json:"receiveEmail"`
		Portal           string `json:"portal"`
		NotificationList string `json:"notificationList"`
	} `json:"url"`
}

type LoginInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	Success bool `json:"success"`
}

type ReqeustUpdatesInfo struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Updates struct {
	Success bool `json:"success"`
	Mail    []Mail
}

type Mail struct {
	Id         int    `json:"id"`
	SenderName string `json:"senderName"`
	Title      string `json:"title"`
}

type History struct {
	Ids []int `json:"ids"`
}
