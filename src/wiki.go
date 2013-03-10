// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
  "fmt"
  "bytes"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
)

type Page struct {
	Title string
	Body  []byte
}

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func loadPage(title string) (*Page, error) {
	filename := "data/" + title + ".txt"
	body, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return &Page{Title: title, Body: body}, nil
}

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

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

  var output bytes.Buffer
  err = templates.ExecuteTemplate(&output, "view.html", p)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }

  w.Write(linkMatcher.ReplaceAllFunc(output.Bytes(), replaceLinks))
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title)
	if err != nil {
		p = &Page{Title: title}
	}
  err = templates.ExecuteTemplate(w, "edit.html", p)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
  }
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")
	p := &Page{Title: title, Body: []byte(body)}
	err := p.save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

var templates = template.Must(template.ParseFiles("tmpl/edit.html", "tmpl/view.html", "tmpl/link.html"))

var titleValidator = regexp.MustCompile("^[a-zA-Z0-9]+$")

func handleFunc (path string, fn func(http.ResponseWriter, *http.Request, string)) {
  lenPath := len(path)
  handler := func(w http.ResponseWriter, r *http.Request) {
    title := r.URL.Path[lenPath:]
    if !titleValidator.MatchString(title) {
      http.NotFound(w, r)
      return
    }
    fn(w, r, title)
	}

  http.HandleFunc(path, handler)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
  http.Redirect(w, r, "/view/main", http.StatusFound)
}

func main() {
  fmt.Println("starting")
  http.HandleFunc("/", rootHandler)
  handleFunc("/view/", viewHandler)
  handleFunc("/edit/", editHandler)
  handleFunc("/save/", saveHandler)
	http.ListenAndServe(":8080", nil)
}
