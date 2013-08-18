// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
  "fmt"
  "bytes"
    "html/template"
    "io"
    "io/ioutil"
    "net/http"
    "net/url"
    "time"
    "container/list"
    "encoding/json"
    "regexp"
    "strconv"
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
    AccessToken    string
    RefreshToken    string
    EventId        string
    RsvpTime       time.Time
}

type AuthBlock struct {
    Token_Type    string "token_type"
    Refresh_Token string "refresh_token"
    Access_Token  string "access_token"
    Expires_In    int    "expires_in"
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


func getEventRsvpTime(eventId string, accessToken string) time.Time {
    v := url.Values{}
    v.Set("event_id", eventId)
    v.Set("access_token", accessToken)
    eventsStr := fmt.Sprintf(
        "https://api.meetup.com/2/events/?access_token=%v&event_id=%v&fields=rsvp_rules", 
        accessToken, eventId)
    resp, _ := http.Get(eventsStr)
    defer resp.Body.Close()
    contents, _ := ioutil.ReadAll(resp.Body)
    re, _ := regexp.Compile("\"open_time\"\\s*:\\s*([0-9]+)")
    matches := re.FindSubmatch(contents)
    t, _ := strconv.ParseInt(string(matches[1]),0, 64)
    return time.Unix(t/1000,0)
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
            var authblock AuthBlock
            jd := json.NewDecoder(resp.Body)
            jd.Decode(&authblock)
            rsvpTime := getEventRsvpTime(r.URL.Query().Get("state"), authblock.Access_Token)
            ch <- RsvpRequest{
                   RequestTime: time.Now(),
                   TokenExpireTime: time.Now().Add(
                    time.Duration(1000*1000*1000*int64(authblock.Expires_In))),
                   AccessToken: authblock.Access_Token,
                   RefreshToken: authblock.Refresh_Token,
                   RsvpTime: rsvpTime,
                   EventId: r.URL.Query().Get("state") }
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
        fmt.Println(rsvp)
        if mintime.Before(rsvp.RsvpTime) {
            mintime = rsvp.RsvpTime
            nextEvent = e
        }
    }
    
    if nextEvent != nil {
        fmt.Println("waiting")
        fmt.Println(mintime.Sub(time.Now()))
        select {
            case <- time.After(mintime.Sub(time.Now())):
        }
        waitch <- nextEvent.Value.(RsvpRequest)
        fmt.Println(nextEvent)
        rsvpList.Remove(nextEvent)
    }
}

func rsvpToEvent(event RsvpRequest) {
    fmt.Println("got event")
    fmt.Println(event)
}

func rsvpTask(ch chan RsvpRequest) {
       
    // Read prev rsvp requests if we crashed
    waitch := make(chan RsvpRequest)
    rsvpList := list.New()
    for {
        select {
            case rsvp := <-ch: {
                rsvp.RsvpTime = rsvp.RsvpTime.Add(1000*1000*1000*1000)
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
    clientSecret  := "pj1gksajgksruc39gjh81udebt"
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
