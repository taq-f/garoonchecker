package connector

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Garoon struct {
	jars   map[string]http.CookieJar
	ticket map[string]string
	config Config
}

type Config struct {
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

func (g *Garoon) Initialize(config Config) {
	g.config = config
	// Cookie jar
	g.jars = map[string]http.CookieJar{"api": nil, "web": nil}
	g.jars["api"], _ = cookiejar.New(nil)
	g.jars["web"], _ = cookiejar.New(nil)
}

func (g *Garoon) Connect() bool {
	// Login to service
	retApi := g.loginApi()
	retWeb := g.loginWeb()

	if !(retApi && retWeb) {
		return false
	}

	// get ticket used to receive email
	g.ticket = g.getTicket()

	return true
}

func (g *Garoon) loginApi() bool {
	username := g.config.Account.Username
	password := g.config.Account.Password

	data := LoginInfo{username, password}
	b, _ := json.Marshal(data)

	client := &http.Client{
		Jar: g.jars["api"],
	}

	resp, _ := client.Post(
		g.config.Url.LoginApi,
		"application/json",
		bytes.NewBuffer(b),
	)
	defer resp.Body.Close()
	byteArray, _ := ioutil.ReadAll(resp.Body)
	result := new(LoginResult)

	json.Unmarshal(byteArray, result)

	return result.Success
}

func (g *Garoon) loginWeb() bool {
	username := g.config.Account.Username
	password := g.config.Account.Password

	client := &http.Client{
		Jar: g.jars["web"],
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Add("_account", username)
	data.Add("_password", password)

	resp, _ := client.Post(
		g.config.Url.LoginWeb,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	defer resp.Body.Close()

	return resp.StatusCode == 302
}

func (g *Garoon) getTicket() map[string]string {
	client := &http.Client{
		Jar: g.jars["web"],
	}

	resp, _ := client.Get(
		g.config.Url.Portal,
	)
	defer resp.Body.Close()

	doc, _ := goquery.NewDocumentFromResponse(resp)

	t := map[string]string{}

	doc.Find("form[name^=mail_receive] input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		// fmt.Println(s)
		val, _ := s.Attr("value")
		name, _ := s.Attr("name")
		t[name] = val
	})

	return t
}

func (g *Garoon) receiveMail() bool {
	client := &http.Client{
		Jar: g.jars["web"],
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	data := url.Values{}
	data.Add("csrf_ticket", g.ticket["csrf_ticket"])
	data.Add("aid", "264")
	data.Add("cmd", g.ticket["cmd"])

	resp, _ := client.Post(
		g.config.Url.ReceiveEmail,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	defer resp.Body.Close()

	return resp.StatusCode == 302
}

func (g *Garoon) GetUpdates() *Updates {
	g.receiveMail()

	client := &http.Client{
		Jar: g.jars["api"],
	}

	t := time.Now().UTC()
	const l = "2006-01-02T15:04:05Z"

	st := t.Add(-24 * time.Hour * 7)
	ed := t

	data := ReqeustUpdatesInfo{Start: st.Format(l), End: ed.Format(l)}
	b, _ := json.Marshal(data)

	resp, _ := client.Post(
		g.config.Url.NotificationList,
		"application/json",
		bytes.NewBuffer(b),
	)
	byteArray, _ := ioutil.ReadAll(resp.Body)
	result := new(Updates)

	json.Unmarshal(byteArray, result)
	defer resp.Body.Close()

	return result
}
