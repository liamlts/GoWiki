package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gomarkdown/markdown"
	"github.com/microcosm-cc/bluemonday"

	"log"

	"net/http"

	"os"

	"regexp"
)

type Page struct {
	Title string

	Body []byte
}

func (p *Page) save() error {

	filename := "pages/" + p.Title + ".txt"

	return os.WriteFile(filename, p.Body, 0600)

}

func loadPage(title string) (*Page, error) {

	filename := "pages/" + title + ".txt"

	body, err := os.ReadFile(filename)

	if err != nil {

		return nil, err

	}

	return &Page{Title: title, Body: body}, nil

}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {

	p, err := loadPage(title)

	if err != nil {

		http.Redirect(w, r, "/edit/"+title, http.StatusFound)

		return

	}

	renderTemplate(w, "view", p)

}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {

	p, err := loadPage(title)

	if err != nil {

		p = &Page{Title: title}

	}

	renderTemplate(w, "edit", p)

}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {

	str_body := r.FormValue("body")
	body := []byte(str_body)
	html_body := markdown.ToHTML(body, nil, nil)
	safe_html := bluemonday.UGCPolicy().SanitizeBytes(html_body)

	p := &Page{Title: title, Body: safe_html}

	err := p.save()

	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)

		return

	}

	http.Redirect(w, r, "/view/"+title, http.StatusFound)

}

var templates = template.Must(template.ParseFiles("edit.html", "view.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {

	err := templates.ExecuteTemplate(w, tmpl+".html", p)

	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)

	}

}

var validPath = regexp.MustCompile("^/(edit|save|view|new)/([A-Za-z0-9_-]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		m := validPath.FindStringSubmatch(r.URL.Path)

		if m == nil {

			http.NotFound(w, r)

			return

		}

		fn(w, r, m[2])

	}

}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fp := filepath.Join("static", "index.html")
	http.ServeFile(w, r, fp)

}

func newPageHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method == "GET" {
		http.ServeFile(w, r, "new.html")
	} else if r.Method == "POST" {
		var sslice []string = r.Form["pagename"]
		s := strings.Join(sslice, "")
		var p *Page
		s = strings.ReplaceAll(s, " ", "_")
		p = &Page{Title: s}
		fmt.Println("Words:", s, "|Slice:", sslice)
		renderTemplate(w, "edit", p)

		fmt.Println("pg:", r.Form["pagename"])
	}
}

func main() {

	http.HandleFunc("/", homeHandler)

	http.HandleFunc("/new", newPageHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.HandleFunc("/view/", makeHandler(viewHandler))

	http.HandleFunc("/edit/", makeHandler(editHandler))

	http.HandleFunc("/save/", makeHandler(saveHandler))

	log.Fatal(http.ListenAndServe("127.0.0.1:7000", nil))

}
