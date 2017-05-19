package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path"
	"strings"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/robfig/cron"
	"github.com/taq-f/garoonchecker/connector"
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

	connectorConfig := connector.Config{}
	connectorConfig.Account.Username = config.Garoon.Account.Username
	connectorConfig.Account.Password = config.Garoon.Account.Password
	connectorConfig.Url.LoginWeb = config.Garoon.Url.LoginWeb
	connectorConfig.Url.LoginApi = config.Garoon.Url.LoginApi
	connectorConfig.Url.ReceiveEmail = config.Garoon.Url.ReceiveEmail
	connectorConfig.Url.Portal = config.Garoon.Url.Portal
	connectorConfig.Url.NotificationList = config.Garoon.Url.NotificationList

	conn := new(connector.Garoon)
	conn.Initialize(connectorConfig)
	conn.Connect()

	c := cron.New()
	var intervals string
	if config.Debug {
		intervals = "*/5 * * * * *"
	} else {
		intervals = "0 */3 * * * *"
	}
	c.AddFunc(intervals, func() {
		updates := conn.GetUpdates()
		filtered := filterByHistory(updates)

		t := time.Now()
		const l = "2006-01-02 15:04:05"

		fmt.Println(t.Format(l) + "-----------------------------")
		for i := 0; i < len(filtered); i++ {
			fmt.Println(filtered[i])
		}

		notify(filtered)
	})
	c.Start()

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

	if exists.File(historyFile) {
		// delete the file if exists
		os.Remove(historyFile)
	}

	file, err := os.Create(historyFile)

	if err != nil {
		fmt.Println("error on opening history file")
	}
	defer file.Close()

	inititalHistory := new(History)
	inititalHistory.Ids = []int{}
	toWrite, _ := json.Marshal(inititalHistory)

	file.Write(toWrite)

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

func filterByHistory(updates *connector.Updates) []*Notification {
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
	filteredUpdates := []*Notification{}

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
			n := new(Notification)
			n.Id = u.Id
			n.Content = u.Title
			filteredUpdates = append(filteredUpdates, n)
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

	return filteredUpdates
}

func notify(notifications []*Notification) {
	contents := []string{}
	for i := 0; i < len(notifications); i++ {
		contents = append(contents, notifications[i].Content)
	}

	if len(notifications) == 0 {
		return
	}

	data := map[string]string{"text": strings.Join(contents, "\n")}

	b, err := json.Marshal(data)
	if err != nil {
		fmt.Println("ERR", err)
		return
	}

	client := &http.Client{}

	resp, _ := client.Post(
		config.Notification.Slack.Url,
		"application/json",
		bytes.NewBuffer(b),
	)
	defer resp.Body.Close()
}

type Config struct {
	Debug  bool `json:"debug"`
	Garoon struct {
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
	} `json:"garoon"`
	Notification struct {
		Slack struct {
			Url string `json:"url"`
		} `json:"slack"`
	} `json:"notification"`
}

type History struct {
	Ids []int `json:"ids"`
}

type Notification struct {
	Id      int
	Title   string
	Content string
}
