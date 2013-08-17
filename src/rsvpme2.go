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
    "regexp"
)


//func (p *Page) save() error {
//    filename := "data/" + p.Title + ".txt"
//    return ioutil.WriteFile(filename, p.Body, 0600)
//}
//
//func loadPage(title string) (*Page, error) {
//    filename := "data/" + title + ".txt"
//    body, err := ioutil.ReadFile(filename)
//    if err != nil {
//        return nil, err
//    }
//    return &Page{Title: title, Body: body}, nil
//}

var linkMatcher = regexp.MustCompile(`\[[a-zA-Z0-9]+\]`)

func replaceLinks(match []byte) []byte {
  last := len(match) - 1
  page := match[1:last]

  var w bytes.Buffer
  err := templates.ExecuteTemplate(&w, "link.html", string(page))
  if err != nil {
    return match
  }
  return []byte(w.String())
}

func requestAuthHandler(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "https://secure.meetup.com/oauth2/authorize?client_id=1q436aibkm3lpb5daoioul87tk&response_type=code&redirect_uri=http://127.0.0.1:8080/authed", http.StatusFound)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
    accessTokenUrl := "https://secure.meetup.com/oauth2/access"
    clientId       := "1q436aibkm3lpb5daoioul87tk"
    clientSecret   := "pj1gksajgksruc39gjh81udebt"
    grantType      := "authorization_code"
    redirectUri    := "http://127.0.0.1:8080/authed"
    code           := r.URL.Query().Get("code")
    v := url.Values{}
    v.Set("client_id", clientId)
    v.Set("client_secret", clientSecret)
    v.Set("grant_type",grantType)
    v.Set("redirect_uri", redirectUri)
    v.Set("code", code)
    resp, err:=http.PostForm(accessTokenUrl, v)
    fmt.Println(resp)
    fmt.Println(err)
}

var templates = template.Must(template.ParseFiles("tmpl/home.html", "tmpl/wegoit.html"))

//func handleFunc (path string, fn func(http.ResponseWriter, *http.Request, string)) {
//  lenPath := len(path)
//  handler := func(w http.ResponseWriter, r *http.Request) {
//    title := r.URL.Path[lenPath:]
//    if !titleValidator.MatchString(title) {
//      http.NotFound(w, r)
//      return
//    }
//    fn(w, r, title)
//    }
//
//  http.HandleFunc(path, handler)
//}

type HomePage struct {
    requestUri string
}

func addRootHandler(uri string) {
    homePage := new(bytes.Buffer)
    err:=templates.ExecuteTemplate(homePage, "home.html", HomePage{requestUri: uri})
    fmt.Println(err)
    handler := func(w http.ResponseWriter, r *http.Request) {
        io.Copy(w, homePage)
    }
    fmt.Println(homePage)
    http.HandleFunc("/", handler)
}

func main() {
    fmt.Println("starting")
    clientId       := "1q436aibkm3lpb5daoioul87tk"
    //grantType      := "authorization_code"
    redirectUri    := "http://127.0.0.1:8080/authed"
    requestUri := fmt.Sprintf("https://secure.meetup.com/oauth2/authorize?client_id=%v&response_type=code&redirect_uri=%v", clientId, redirectUri)
    //homePage = templates.ExecuteTemplate(w, "home.html",  HomePage{requestUri: requestUri})
    fmt.Println(requestUri)
    addRootHandler(requestUri)
    http.HandleFunc("/add", requestAuthHandler)
    http.HandleFunc("/authed", authHandler)
    http.ListenAndServe(":8080", nil)
}
