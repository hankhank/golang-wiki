// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
  "fmt"
  "bytes"
    "html/template"
    "io"
    "net/http"
    "net/url"
    "time"
    "container/list"
)

type HomePage struct {
    RequestUri string
    RefererUri string
    ClientId string
}

type WeGotItPage struct {
    EventInfo string
}

type RsvpRequest struct {
    RequestTime     time.Time
    TokenExpireTime time.Time
    AcccessToken    string
    RefreshToken    string
    EventUrl        string
    RsvpTime       time.Time
}

var templates = template.Must(template.ParseFiles("tmpl/home.html", "tmpl/wegotit.html"))


func addRootHandler(uri string, refererUri string, clientId string) {
    homePage := new(bytes.Buffer)
    templates.ExecuteTemplate(homePage, "home.html", HomePage{RequestUri: uri, RefererUri: refererUri, ClientId: clientId})
    handler := func(w http.ResponseWriter, r *http.Request) {
        io.Copy(w, bytes.NewBuffer(homePage.Bytes()))
    }
    fmt.Println(homePage)
    http.HandleFunc("/", handler)
}

func addAuthHandler(ch chan RsvpRequest, accessUrl string, refererUri string, clientId string, clientSecret string) {
    
    handler := func(w http.ResponseWriter, r *http.Request) {
        code := r.URL.Query().Get("code")
        v := url.Values{}
        v.Set("client_id", clientId)
        v.Set("client_secret", clientSecret)
        v.Set("grant_type", "authorization_code")
        v.Set("redirect_uri", refererUri)
        v.Set("code", code)
        resp, err := http.PostForm(accessUrl, v)
        if (err == nil && resp.StatusCode == http.StatusOK) {
            buf := new(bytes.Buffer)
            defer resp.Body.Close()
            io.Copy(buf, resp.Body)
            ch <- RsvpRequest{EventUrl: r.URL.Query().Get("state")}
            http.Redirect(w, r, "/wegotit", http.StatusFound)
        } else {
            http.Redirect(w, r, "/fark", http.StatusFound)
        }
    }
    http.HandleFunc("/authed/", handler)
}

func weGotIt(w http.ResponseWriter, r *http.Request) {
    templates.ExecuteTemplate(w, "wegotit.html", WeGotItPage{EventInfo: ""})
}

func startWait(waitch chan RsvpRequest, rsvpList *list.List) {

    mintime := time.Now()
    nextEvent := rsvpList.Front()

    for e := rsvpList.Front(); e != nil; e = e.Next() {
        rsvp := e.Value.(RsvpRequest)
        if mintime.Before(rsvp.RsvpTime) {
            mintime = rsvp.RsvpTime
            nextEvent = e
        }
    }
    
    ch := time.After(mintime.Sub(time.Now()))
    select {
        case <- ch:
    }
    waitch <- nextEvent.Value.(RsvpRequest)
    fmt.Println(nextEvent)
    rsvpList.Remove(nextEvent)
}

func rsvpToEvent(event RsvpRequest) {
}

func rsvpTask(ch chan RsvpRequest) {
       
    // Read prev rsvp requests if we crashed
    waitch := make(chan RsvpRequest)
    rsvpList := list.New()
    for {
        select {
            case rsvp := <-ch: {
                rsvpList.PushBack(rsvp)
            }
            case event := <- waitch: {
                rsvpToEvent(event)
            }
        }
        fmt.Println("asd")
        go startWait(waitch, rsvpList)
    }
}

func main() {
    fmt.Println("starting")
    clientId      := "1q436aibkm3lpb5daoioul87tk"
    redirectUri   := "http://127.0.0.1:8080/authed/"
    authorizeUrl  := "https://secure.meetup.com/oauth2/authorize"
    accessUrl   := "https://secure.meetup.com/oauth2/access"
    
    ch := make(chan RsvpRequest)
    go rsvpTask(ch)

    addRootHandler(authorizeUrl, redirectUri, clientId)
    addAuthHandler(ch, accessUrl, redirectUri, clientId, clientSecret)
    http.HandleFunc("/wegotit", weGotIt)
    http.ListenAndServe(":8080", nil)
}
