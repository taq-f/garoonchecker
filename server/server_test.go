package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestLoginApi(t *testing.T) {
	go Start()

	testLoginApi(t)
	testLoginWeb(t)
	testPortal(t)
	testReceiveEmail(t)
	testNotificationList(t)
	testNotificationWebHook(t)
}

func testLoginApi(t *testing.T) {
	reqUrl := "http://localhost:3000/loginApi"

	var result struct {
		Success bool `json:"success"`
	}
	var b []byte
	var data map[string]interface{}

	// currect login information
	data = map[string]interface{}{
		"username": "foo",
		"password": "bar",
	}
	b = requestJson(reqUrl, data)
	json.Unmarshal(b, &result)

	if !result.Success {
		t.Errorf("login api unexpectedly faild.")
	}

	// wrong login information
	data = map[string]interface{}{
		"username": "foo?",
		"password": "bar",
	}
	b = requestJson(reqUrl, data)
	json.Unmarshal(b, &result)

	if result.Success {
		t.Errorf("login api unexpectedly faild")
	}

	// wrong login information
	data = map[string]interface{}{
		"username": "foo",
		"password": "bar?",
	}
	b = requestJson(reqUrl, data)
	json.Unmarshal(b, &result)

	if result.Success {
		t.Errorf("login api unexpectedly faild")
	}

	// wrong login information
	data = map[string]interface{}{
		"username": "foo?",
		"password": "bar?",
	}
	b = requestJson(reqUrl, data)
	json.Unmarshal(b, &result)

	if result.Success {
		t.Errorf("login api unexpectedly faild")
	}
}

func testLoginWeb(t *testing.T) {
	reqUrl := "http://localhost:3000/loginWeb"
	var statusCode int

	// correct login information
	correctInfo := url.Values{}
	correctInfo.Add("_account", "foo")
	correctInfo.Add("_password", "bar")

	statusCode = post(reqUrl, correctInfo)

	if statusCode != 302 {
		t.Errorf("login web unexpectedly faild")
	}

	// wrong login information
	wrongInfo := url.Values{}
	wrongInfo.Add("_account", "foo?")
	wrongInfo.Add("_password", "bar")

	statusCode = post(reqUrl, wrongInfo)

	if statusCode == 302 {
		t.Errorf("login web unexpectedly succeeded")
	}
}

func testPortal(t *testing.T) {
	reqUrl := "http://localhost:3000/portal"
	client := &http.Client{}
	var resp *http.Response
	var ticket map[string]string
	var doc *goquery.Document

	// without error
	resp, _ = client.Get(
		reqUrl,
	)
	defer resp.Body.Close()

	ticket = map[string]string{}
	doc, _ = goquery.NewDocumentFromResponse(resp)

	doc.Find("form[name^=mail_receive] input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		// fmt.Println(s)
		val, _ := s.Attr("value")
		name, _ := s.Attr("name")
		ticket[name] = val
	})

	if ticket["csrf_ticket"] != "some_ticket" || ticket["cmd"] != "some_cmd" {
		t.Errorf("invalid portal")
	}

	// with error
	resp, _ = client.Get(
		reqUrl + "?error=true",
	)

	defer resp.Body.Close()

	ticket = map[string]string{}
	doc, _ = goquery.NewDocumentFromResponse(resp)

	doc.Find("form[name^=mail_receive] input[type=hidden]").Each(func(_ int, s *goquery.Selection) {
		// fmt.Println(s)
		val, _ := s.Attr("value")
		name, _ := s.Attr("name")
		ticket[name] = val
	})

	if ticket["csrf_ticket"] == "some_ticket" && ticket["cmd"] == "some_cmd" {
		t.Errorf("ticket should not be retrieved")
	}
}

func testReceiveEmail(t *testing.T) {
	reqUrl := "http://localhost:3000/receiveEmail"
	var statusCode int

	statusCode = post(reqUrl, nil)

	if statusCode != 302 {
		t.Errorf("receive mail request unexpectedly faild")
	}
}

func testNotificationList(t *testing.T) {
	reqUrl := "http://localhost:3000/notificationList"

	var result struct {
		Success bool `json:"success"`
		Mail    []struct {
			Id         int    `json:"id"`
			SenderName string `json:"senderName"`
			Title      string `json:"title"`
		} `json:"mail"`
	}
	var b []byte

	// get mail updates
	b = requestJson(reqUrl, nil)
	json.Unmarshal(b, &result)

	if !result.Success {
		t.Errorf("got fail response")
	}
	if len(result.Mail) != 4 {
		t.Errorf("unexpected number of updates. expected %v, got %v", 4, len(result.Mail))
	}
}

func testNotificationWebHook(t *testing.T) {
	reqUrl := "http://localhost:3000/notificationWebHook"
	statusCode := post(reqUrl, nil)

	if statusCode != 200 {
		t.Errorf("notification request unexpectedly failed")
	}
}

// ****************************************************************************
// Helpers
// ****************************************************************************

func requestJson(url string, data interface{}) []byte {
	b, _ := json.Marshal(&data)

	client := &http.Client{}
	resp, _ := client.Post(
		url,
		"application/json",
		bytes.NewBuffer(b),
	)
	defer resp.Body.Close()
	byteArray, _ := ioutil.ReadAll(resp.Body)

	return byteArray
}

func post(url string, v url.Values) int {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, _ := client.Post(
		url,
		"application/x-www-form-urlencoded",
		strings.NewReader(v.Encode()),
	)
	defer resp.Body.Close()

	return resp.StatusCode
}
