package main

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"text/template"
	"time"

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

	User User
}

type User struct {
	Name string
}

var curUser string

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
	str_html := string(safe_html) + "\n"
	html_full := []byte(str_html)

	p := &Page{Title: title, Body: html_full}

	err := p.save()

	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)

		return

	}

	http.Redirect(w, r, "/view/"+title, http.StatusFound)

}

var templates = template.Must(template.ParseFiles("edit.html", "view.html", "home.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *Page) {

	err := templates.ExecuteTemplate(w, tmpl+".html", p)

	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)

	}

}

var validPath = regexp.MustCompile("^/(edit|save|view|new|random|login)/([A-Za-z0-9_-]+)$")

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

func search(query string) (string, error) {
	pages, err := ioutil.ReadDir("pages/")
	if err != nil {
		log.Fatal(err)
	}

	var pagelist []string

	for _, page := range pages {
		pagelist = append(pagelist, page.Name())
	}

	for i := range pagelist {
		pagelist[i] = pagelist[i][:len(pagelist[i])-len(".txt")]
	}

	for n := range pagelist {
		if strings.Contains(pagelist[n], query) {
			return pagelist[n], nil
		}
	}
	return "", err
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method == "GET" {
		if curUser == "" {
			curUser = "Not Logged in"
			user := User{Name: curUser}
			err := templates.ExecuteTemplate(w, "home.html", user)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			user := User{Name: curUser}
			err := templates.ExecuteTemplate(w, "home.html", user)
			if err != nil {
				log.Fatal(err)
			}
		}
	} else if r.Method == "POST" {
		query := r.Form["search"]
		user_search := strings.Join(query, "")
		user_search = strings.ReplaceAll(user_search, " ", "_")
		fmt.Println(user_search)

		pagename, err := search(user_search)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if pagename != "" {
			p, err := loadPage(pagename)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
			}
			renderTemplate(w, "view", p)

		} else {
			var user User
			if curUser == "" {
				user = User{Name: "not logged in"}
			} else {
				user = User{Name: curUser}
			}
			err := templates.ExecuteTemplate(w, "home.html", user)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func exists() bool {
	_, err := os.Stat("users/")
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}

func logindir() {
	if !exists() {
		err := os.Mkdir("users", 0744)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func hasAccount(uname string) bool {
	_, err := os.Stat("users/" + uname + ".data")
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return false
}

func makeAccount(uname string, pword string) {
	user, err := os.OpenFile("users/"+uname+".data",
		os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}

	hasher := sha512.New()

	bpass := []byte(pword)

	hash := hasher.Sum(bpass)

	if _, err := user.WriteString(hex.EncodeToString(hash)); err != nil {
		log.Fatal(err)
	}

}

func loginSuccess(username string, pass string) bool {
	uname := "users/" + username + ".data"
	hpass, err := ioutil.ReadFile(uname)
	if err != nil {
		log.Fatal(err)
	}
	hasher := sha512.New()

	upass := []byte(pass)

	rhash := hasher.Sum(upass)
	srhash := hex.EncodeToString(rhash)

	return srhash == string(hpass)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		http.ServeFile(w, r, "login.html")
	} else if r.Method == "POST" {
		r.ParseForm()
		uname := r.Form["username"]
		pword := r.Form["password"]
		if !hasAccount(uname[0]) {
			makeAccount(uname[0], pword[0])
		} else if loginSuccess(uname[0], pword[0]) {
			exp := time.Now().Add(365 * 24 * time.Hour)
			cookie := http.Cookie{Name: "username", Value: uname[0], Expires: exp}
			http.SetCookie(w, &cookie)
			r.AddCookie(&cookie)
			val, err := r.Cookie("username")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(val)
			//personname := val.Value
			user := User{Name: val.Value}
			curUser = val.Value
			//w.Write([]byte("login successful, welcome " + user.Name))
			err = templates.ExecuteTemplate(w, "home.html", user)
			if err != nil {
				log.Fatal(err)
			}
		} else if !loginSuccess(uname[0], pword[0]) {
			w.Write([]byte("invalid password and or username"))
		}

	}
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

func GetRandomPage() (*Page, error) {
	pages, err := ioutil.ReadDir("pages/")
	if err != nil {
		log.Fatal(err)
	}
	var pagenames []string

	for _, page := range pages {
		pagenames = append(pagenames, page.Name())
	}

	rPage := rand.Intn(len(pagenames))
	extPage := pagenames[rPage]
	nPage := extPage[:len(extPage)-len(".txt")]
	fmt.Println(nPage)
	p, err := loadPage(nPage)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func randomHandler(w http.ResponseWriter, r *http.Request) {
	p, err := GetRandomPage()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	renderTemplate(w, "view", p)

}

func main() {
	logindir()

	http.HandleFunc("/", homeHandler)

	http.HandleFunc("/new", newPageHandler)

	http.HandleFunc("/random", randomHandler)

	http.HandleFunc("/login", loginHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.HandleFunc("/view/", makeHandler(viewHandler))

	http.HandleFunc("/edit/", makeHandler(editHandler))

	http.HandleFunc("/save/", makeHandler(saveHandler))

	srv := &http.Server{
		Addr: "127.0.0.1:7000",

		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
