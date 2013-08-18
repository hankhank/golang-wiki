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
    "strings"
)

var MeetupUrl     = string("https://api.meetup.com/2")
var AuthorizeUrl  = string("https://secure.meetup.com/oauth2")

var templates = template.Must(template.ParseFiles("tmpl/home.html", "tmpl/wegotit.html"))

type HomePage struct {
    RequestUri string
    RefererUri string
    ClientId   string
}

type WeGotItPage struct {
    EventInfo string
}

type AuthBlock struct {
    TokenType    string `json:"token_type"`
    RefreshToken string `json:"refresh_token"`
    AccessToken  string `json:"access_token"`
    ExpiresIn    int    `json:"expires_in"`
    ExpireTime   time.Time
}

type RsvpRequest struct {
    RequestTime  time.Time
    EventId      string
    RsvpTime     time.Time
    AuthBlock    AuthBlock
}

func addRootHandler(refererUri string, clientId string) {

    homePage := new(bytes.Buffer)

    templates.ExecuteTemplate(homePage, "home.html", 
        HomePage{RequestUri: fmt.Sprintf("%v/authorize",AuthorizeUrl),
        RefererUri: refererUri, ClientId: clientId})

    fmt.Println(homePage)

    handler := func(w http.ResponseWriter, r *http.Request) {
        io.Copy(w, bytes.NewBuffer(homePage.Bytes()))
    }

    http.HandleFunc("/", handler)
}

func getEventRsvpTime(eventId string, accessToken string) time.Time {

    eventsStr := fmt.Sprintf("%v/events?&access_token=%v&event_id=%v&fields=rsvp_rules", 
        MeetupUrl, accessToken, eventId)

    resp, _ := http.Get(eventsStr)
    defer resp.Body.Close()
    contents, _ := ioutil.ReadAll(resp.Body)

    re, _ := regexp.Compile("\"open_time\"\\s*:\\s*([0-9]+)")
    matches := re.FindSubmatch(contents)

    t, _ := strconv.ParseInt(string(matches[1]),0, 64)
    return time.Unix(t/1000,0)
}

func addAuthHandler(ch chan RsvpRequest, refererUri string, clientId string, consumerSecret string) {
    
    handler := func(w http.ResponseWriter, r *http.Request) {

        // Request Auth from meetup
        code := r.URL.Query().Get("code")
        v := url.Values{}
        v.Set("client_id",     clientId)
        v.Set("client_secret", consumerSecret)
        v.Set("grant_type",   "authorization_code")
        v.Set("redirect_uri",  refererUri)
        v.Set("code",          code)
        resp, err := http.PostForm(fmt.Sprintf("%v/access", AuthorizeUrl), v)

        // Once we have it parse it down and create an rsvp task
        if (err == nil && resp.StatusCode == http.StatusOK) {

            var authblock AuthBlock
            jd := json.NewDecoder(resp.Body)
            jd.Decode(&authblock)

            authblock.ExpireTime = time.Now().Add(
                    time.Duration(1000*1000*1000*int64(authblock.ExpiresIn)))
            rsvpTime := getEventRsvpTime(r.URL.Query().Get("state"), authblock.AccessToken)

            // Signal the rsvp handler
            ch <- RsvpRequest{
                   RequestTime: time.Now(),
                   RsvpTime:    rsvpTime,
                   EventId:     r.URL.Query().Get("state"),
                   AuthBlock:   authblock}

            // Let the customer know everythings going to be fine
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

func startWait(WaitCh chan RsvpRequest, rsvpList *list.List) {

    mintime   := time.Now()
    nextEvent := rsvpList.Front()

    for e := rsvpList.Front(); e != nil; e = e.Next() {
        rsvp := e.Value.(RsvpRequest)
        if mintime.Before(rsvp.RsvpTime) {
            mintime = rsvp.RsvpTime
            nextEvent = e
        }
    }
    
    if nextEvent != nil {
        select {
            case <- time.After(mintime.Sub(time.Now())):
            fmt.Sprint(nextEvent.Value.(RsvpRequest))
        }
        WaitCh <- nextEvent.Value.(RsvpRequest)
        rsvpList.Remove(nextEvent)
    }
}

func refreshAuth(event RsvpRequest, clientId string, consumerSecret string) {

    v := url.Values{}
    v.Set("client_id",     clientId)
    v.Set("client_secret", consumerSecret)
    v.Set("grant_type",   "refresh_token")
    v.Set("refresh_token", event.AuthBlock.RefreshToken)
    resp, err := http.PostForm(fmt.Sprintf("%v/access", AuthorizeUrl), v)

    if (err == nil && resp.StatusCode == http.StatusOK) {
        var authblock AuthBlock
        jd := json.NewDecoder(resp.Body)
        jd.Decode(&authblock)
        authblock.ExpireTime = time.Now().Add(
            time.Duration(1000*1000*1000*int64(authblock.ExpiresIn)))
        event.AuthBlock = authblock
    }
}

func rsvpToEvent(event RsvpRequest, clientId string, consumerSecret string) {
    
    refreshAuth(event, clientId, consumerSecret)

    v := url.Values{}
    v.Set("event_id", event.EventId)
    v.Set("rsvp",    "yes")

    client := &http.Client{}
    req, _ := http.NewRequest("POST", fmt.Sprintf("%v/rsvp/", MeetupUrl), strings.NewReader(v.Encode()))
    req.Header.Add("Authorization", fmt.Sprintf(`bearer "%v"`,event.AuthBlock.AccessToken))
    resp, _ := client.Do(req)

    defer resp.Body.Close()
    contents, _ := ioutil.ReadAll(resp.Body)
    fmt.Println("Rvsp now")
    fmt.Println(string(contents))
}

func rsvpTask(ch chan RsvpRequest, clientId string, consumerSecret string) {
       
    WaitCh := make(chan RsvpRequest)
    rsvpList := list.New()
    for {
        // Lets wait on new request and the closest
        // rsvp open time
        select {
            case rsvp := <-ch: {
                rsvpList.PushBack(rsvp)
            }
            case rsvpEvent := <- WaitCh: {
                rsvpToEvent(rsvpEvent, clientId, consumerSecret)
            }
        }
        go startWait(WaitCh, rsvpList)
    }
}

func main() {
    fmt.Println("starting")

    clientId        := ""
    consumerSecret  := ""
    redirectUri     := "http://127.0.0.1:8080/authed/"
    
    ch := make(chan RsvpRequest)
    go rsvpTask(ch, clientId, consumerSecret)

    addRootHandler(redirectUri, clientId)
    addAuthHandler(ch, redirectUri, clientId, consumerSecret)
    http.HandleFunc("/wegotit", weGotIt)
    http.ListenAndServe(":8080", nil)
}
