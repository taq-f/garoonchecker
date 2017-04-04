package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func Start() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":3000", nil)
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/loginWeb":
		handleLoginWeb(w, r)
	case "/loginApi":
		handleLoginApi(w, r)
	case "/receiveEmail":
		handleReceiveEmail(w, r)
	case "/portal":
		handlePortal(w, r)
	case "/notificationList":
		handleNotificationList(w, r)
	case "/notificationWebHook":
		handleNotificationWebHook(w, r)
	}
}

func handleLoginWeb(w http.ResponseWriter, r *http.Request) {

	username := r.FormValue("_account")
	password := r.FormValue("_password")

	if username == "foo" && password == "bar" {
		http.Redirect(w, r, "/portal", http.StatusFound)
	} else {
		fmt.Fprintf(w, "dummy")
	}
}

func handleLoginApi(w http.ResponseWriter, r *http.Request) {
	var f interface{}

	w.Header().Set("Content-Type", "application/json")

	body, _ := ioutil.ReadAll(r.Body)
	json.Unmarshal(body, &f)
	m := f.(map[string]interface{})

	username := m["username"]
	password := m["password"]

	if username == "foo" && password == "bar" {
		fmt.Fprintf(w, "{\"success\": true}")
	} else {
		fmt.Fprintf(w, "{\"success\": false}")
	}
}

func handlePortal(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("error") == "true" {
		w.WriteHeader(500)
	} else {
		fmt.Fprintf(w, `<!DOCTYPE html>
			<html>
			    <head>
			        <meta charset="utf-8">
			        <title></title>
			    </head>
			    <body>
					<form name="mail_receive">
						<input type="hidden" name="csrf_ticket" value="some_ticket" />
						<input type="hidden" name="cmd" value="some_cmd" />
					</form>
			    </body>
			</html>
		`)
	}
}

func handleReceiveEmail(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/portal", http.StatusFound)
}

func handleNotificationList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `
		{
			"success": true,
			"mail": [
				{"id": 100000, "senderName": "100000@example.com", "title": "abount 100000"},
				{"id": 100001, "senderName": "100001@example.com", "title": "abount 100001"},
				{"id": 100002, "senderName": "100002@example.com", "title": "abount 100002"},
				{"id": 100003, "senderName": "100003@example.com", "title": "abount 100003"}
			]
		}
	`)
}

func handleNotificationWebHook(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "dummy")
}
