package main

import (
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
)

type NavItem struct {
	Path string
	Name string
	Title string
}

var NavItems = []NavItem {
    {"/", "Home", "Thomas the (LeMon) Tank Engine"},
	{"/about", "About", "About Thomas the LeMon"},
	{"/dashboard", "Telemetry", "Telemetry Dashboard"},
}

func getCurrentPage(req *http.Request) *NavItem{
	for _, page := range(NavItems) {
		if req.URL.Path == page.Path {
			return &page
		}
	}
	// can't determine the page so return the home page. This should
	// never happen if the routes are set up correctly
	return &NavItems[0]
}

func RegisterSiteHandlers(r *mux.Router) {
    r.HandleFunc("/", HomeHandler)
    r.HandleFunc("/about", AboutHandler)
	r.HandleFunc("/dashboard", TelemetryHandler)
}

func TelemetryHandler(w http.ResponseWriter, req *http.Request) {
	if err := WriteHeader(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	body, err := template.ParseFiles("templates/telemetry.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := body.Execute(w, ""); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := WriteFooter(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func HomeHandler(w http.ResponseWriter, req *http.Request) {
	if err := WriteHeader(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	body, err := template.ParseFiles("templates/home.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := body.Execute(w, ""); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := WriteFooter(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func AboutHandler(w http.ResponseWriter, req *http.Request) {
	if err := WriteHeader(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	body, err := template.ParseFiles("templates/about.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := body.Execute(w, ""); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := WriteFooter(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func WriteHeader(w http.ResponseWriter, req *http.Request) error {
	header, err := template.ParseFiles("templates/header.html", "templates/nav.html")
	if err != nil {
		return err
	}

	current := getCurrentPage(req)

	data := struct {
           CurrentPage *NavItem
		NavItems *[]NavItem
	}{
		current,
		&NavItems,
	}
	return header.Execute(w, data)
}

func WriteFooter(w http.ResponseWriter, req *http.Request) error {
	footer, err := template.ParseFiles("templates/footer.html")
	if err != nil {
		return err
	}
	return footer.Execute(w, "")
}
