package main

import (
	"github.com/gorilla/mux"
	"html/template"
	"net/http"
)

type NavItem struct {
	Path  string
	Name  string
	Title string
}

var NavItems = []NavItem{
	{"/", "Home", "Thomas the (LeMon) Tank Engine"},
	{"/about", "About", "About Thomas the LeMon"},
	{"/dashboard", "Telemetry", "Telemetry Dashboard"},
	{"/dashboard2", "Telemetry 2", "Telemetry Dashboard 2"},
}

type CarouselItem struct {
	Path    string
	Title   string
	Caption string
}

var CarouselItems = []CarouselItem{
	{"/static/photos/car1.jpg", "Day Zero", "After a sight-unseen purchase the 1995 mk3 Volkswagen Jetta completes its first task; getting home regardless of the oil hanging off its engine."},
	{"/static/photos/car2.jpg", "Day Zero", "The first surprise of the purchase was that we hadn't bought a 2.slow, but in fact a VR6 (not that it's fast mind you)."},
	{"/static/photos/car3.jpg", "Light Sunday Drive", "Stripping out all the interior for weight, and selling the many surprising parts lurking in the car kept us in our $500 LeMons budget."},
	{"/static/photos/car4.jpg", "Weekend on Dirt", "Before the final transformation to LeMons extraordinaire (or pile of junk) the car was put through its paces on the dirts of Prairie City."},
	{"/static/photos/car5.jpg", "Final Stage", "Roll cage, racing seat, engine cut-off switch, unexpectedly needed new windshield (whoops) and blue paint."},
	{"/static/photos/car6.jpg", "First Day of Racing", "Location: Sears Point in Sonoma County. Mission: Make it out of the pits to the track."},
	{"/static/photos/car7.jpg", "The Dream is Realized", "Location: On the frickin' track and the engine is still in the car! Mission: Stay on the track."},
	{"/static/photos/car8.jpg", "\"That'll buff out\"", "Mission Failed. The fallen included radiator, wiring, engine mounts, frame and various body panels"},
	{"/static/photos/car9.jpg", "Weld and Straighten", "Everything can be fixed with a bit of welding"},
	{"/static/photos/car10.jpg", "Pick-n-Pull", "This white Jetta never saw us coming. We thank it for its contribution to Thomas' rebirth."},
}

func getCurrentPage(req *http.Request) *NavItem {
	for _, page := range NavItems {
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
	r.HandleFunc("/dashboard2", Telemetry2Handler)
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

func Telemetry2Handler(w http.ResponseWriter, req *http.Request) {
	if err := WriteHeader(w, req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	body, err := template.ParseFiles("templates/telemetry2.html")
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

	data := struct {
		Images *[]CarouselItem
	}{
		&CarouselItems,
	}

	body, err := template.ParseFiles("templates/about.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if err := body.Execute(w, data); err != nil {
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
		NavItems    *[]NavItem
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
