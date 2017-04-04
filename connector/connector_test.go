package connector

import (
	"testing"

	"github.com/taq-f/garoonchecker/server"
)

func TestConnector(t *testing.T) {
	go server.Start()

	username := "foo"
	password := "bar"

	c := Config{}
	c.Account.Username = username
	c.Account.Password = password
	c.Url.LoginWeb = "http://localhost:3000/loginWeb"
	c.Url.LoginApi = "http://localhost:3000/loginApi"
	c.Url.ReceiveEmail = "http://localhost:3000/receiveEmail"
	c.Url.Portal = "http://localhost:3000/portal"
	c.Url.NotificationList = "http://localhost:3000/notificationList"

	g := Garoon{}

	g.Initialize(c)

	if g.config.Account.Username != username {
		t.Errorf("get %v\nwant %v", g.config.Account.Username, username)
	}

	var loginRet bool

	loginRet = g.loginApi()
	if !loginRet {
		t.Errorf("login api unexpectedly faild")
	}

	loginRet = g.loginWeb()
	if !loginRet {
		t.Errorf("login web unexpectedly faild")
	}

	loginRet = g.Connect()
	if !loginRet {
		t.Errorf("connect to service unexpectedly faild")
	}

	// debug web server won't let you login with other than foo/bar
	// in that case, expect result "false"
	g.config.Account.Username = "foo?"

	loginRet = g.loginApi()
	if loginRet {
		t.Errorf("login api unexpectedly succeeded")
	}

	loginRet = g.loginWeb()
	if loginRet {
		t.Errorf("login web unexpectedly succeeded")
	}
	loginRet = g.Connect()
	if loginRet {
		t.Errorf("connect to service unexpectedly succeeded")
	}

	g.config.Url.Portal = "http://localhost:3000/portal?error=true"
	connectRet := g.Connect()
	if connectRet {
		t.Errorf("connect to service unexpectedly succeeded")
	}

	g.config.Url.Portal = "http://localhost:3000/portal"
	g.Connect()
	g.GetUpdates()
}
