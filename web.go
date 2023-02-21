package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Route struct {
	Path  string
	Alias string
}

type Config struct {
	Get  *ConfigGet
	Post *ConfigPost
}

type TemplateFill struct {
	Routes   []Route
	Constant map[string]string
	Query    map[string]string
}

type ConfigGet struct {
	Template string
	Items    TemplateFill
}

type ConfigPost struct {
	Query    string
	Arg      string
	Redirect string
}

func serveVideo(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Path[len("/watch/"):]

		tx, err := db.Begin()
		if err != nil {
			log.Println(err)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare("SELECT name, value FROM tags WHERE filename is ?;")
		if err != nil {
			log.Println(err)
			return
		}

		rows, err := stmt.Query(filename)
		if err != nil {
			log.Println(err)
			return
		}

		tags := make(map[string]string)
		for rows.Next() {
			var k, v string
			err = rows.Scan(&k, &v)
			if err != nil {
				log.Println(err)
				return
			}
			tags[k] = v
		}

		err = tx.Commit()
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func serveThumbs(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Path[5:]

		tx, err := db.Begin()
		if err != nil {
			log.Println(err)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare("SELECT image FROM thumbnails WHERE filename is ?;")
		if err != nil {
			log.Println(err)
			return
		}
		defer stmt.Close()

		var blob []byte
		err = stmt.QueryRow(filename).Scan(&blob)
		if err != nil {
			log.Printf("Wanted thumbnail for '%s', got: %v", filename, err)
			return
		}

		err = tx.Commit()
		if err != nil {
			log.Println(err)
			return
		}

		_, err = w.Write(blob)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func addRoutes(db *sql.DB) ([]Route, error) {
	otherRoutes := make([]Route, 0, 10)

	routes := make(map[string]Config)
	json.Unmarshal([]byte(jsonRoutes), &routes)

	for k, _ := range routes {
		r := Route{Path: k, Alias: ""}
		if k == "/" {
			r.Alias = "home"
		} else {
			r.Alias = k[1:]
		}
		otherRoutes = append(otherRoutes, r)
	}

	for k, cfg := range routes {
		if cfg.Get != nil {
			cfg.Get.Items.Routes = otherRoutes
		}

		handler, err := createSoftServe(cfg, db)
		if err != nil {
			log.Println(err)
			return otherRoutes, err
		}

		log.Printf("Adding handler for '%s' route\n", k)
		http.HandleFunc(k, handler)
	}

	return otherRoutes, nil
}

func createSearchHandler(db *sql.DB, otherRoutes []Route) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		td := make(map[string]interface{})

		err := req.ParseForm()
		if err != nil {
			log.Fatal(err)
		}

		terms := make([]string, 0, 50)
		for _, term := range req.Form["terms"] {
			inQuotes := false
			lo := 0
			i := 0
			for _, c := range term {
				if inQuotes {
					if c == '"' {
						if i > lo {
							terms = append(terms, term[lo:i])
						}
						lo = i+1
						inQuotes = false
					}
				} else if c == '"' {
					if i > lo {
						terms = append(terms, term[lo:i])
					}
					inQuotes = true
					lo = i+1
				} else if c == ' ' {
					if i > lo {
						terms = append(terms, term[lo:i])
					}
					lo = i+1
				} else if c == ':' {
					if i > lo {
						terms = append(terms, term[lo:i+1])
					}
					lo = i+1
				}
				i++
			}

			if i > lo {
				terms = append(terms, term[lo:i])
			}
		}

		log.Printf("Terms:\n")
		for _, term := range terms {
			log.Printf("\t%s\n", term)
		}

		td["terms"] = terms
		td["routes"] = otherRoutes

		params := SearchParameters{
			Vals:        make([]string, 0, 50),
			KeyVals:     make(map[string]string),
			RandomOrder: true,
			Limit:       50,
		}

		skip := false
		for i, term := range terms {
			if skip {
				skip = false
				continue
			}

			if strings.Contains(term, ":") {
				if len(terms) > i+1 {
					params.KeyVals[term[:len(term)-1]] = terms[i+1]
					skip = true
				}
			} else {
				params.Vals = append(params.Vals, term)
			}
		}

		matches, err := lookup2(db, params)
		if err != nil {
			log.Fatal(err)
		}
		td["media"] = matches
		td["count"] = len(matches)

		tmpl, err := loadTemplate("index")
		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, td)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func loadTemplate(name string) (*template.Template, error) {
	rawBase := templates["base"]
	rawTmpl := templates[name]

	tmpl := template.Must(template.New("base").Parse(string(rawBase)))
	template.Must(tmpl.New("body").Parse(string(rawTmpl)))

	return tmpl, nil
}

func emptyHandler(w http.ResponseWriter, req *http.Request) {
	// nothing! wahey
}

func createSoftServe(cfg Config, db *sql.DB) (http.HandlerFunc, error) {
	handleGet := createGetHandler(db, cfg.Get)
	handlePost := createPostHandler(db, cfg.Post)
	return func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "GET":
			handleGet(w, req)
		case "POST":
			handlePost(w, req)
		default:
			fmt.Fprintf(w, "Method not implemented: %s\n", req.Method)
		}
	}, nil
}

func createPostHandler(db *sql.DB, cfg *ConfigPost) func(http.ResponseWriter, *http.Request) {
	if cfg == nil {
		return func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintln(w, "Method not implemented: POST")
		}
	}

	return func(w http.ResponseWriter, req *http.Request) {
		err := req.ParseForm()
		if err != nil {
			log.Println(err)
			return
		}

		tx, err := db.Begin()
		if err != nil {
			log.Println(err)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(cfg.Query)
		defer stmt.Close()

		for _, term := range req.Form[cfg.Arg] {
			log.Printf("%s :: %s\n", cfg.Query, term)
			_, err := stmt.Exec(term)
			if err != nil {
				log.Println(err)
				return
			}
		}

		err = tx.Commit()
		if err != nil {
			log.Println(err)
			return
		}


		redir := cfg.Redirect
		if redir == "" {
			redir = req.URL.Path
		}
		http.Redirect(w, req, redir, http.StatusFound)
	}
}

func createGetHandler(db *sql.DB, cfg *ConfigGet) func(http.ResponseWriter, *http.Request) {
	if cfg == nil {
		return func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintln(w, "Method not implemented: GET")
		}
	}

	return func(w http.ResponseWriter, req *http.Request) {
		err := req.ParseForm()
		if err != nil {
			log.Println(err)
			return
		}

		args := make([]any, 0, 10)
		for _, v := range req.Form["arg"] {
			v, err = url.QueryUnescape(v)
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("Argument: '%v'\n", v)
			args = append(args, v)
		}

		td := make(map[string]interface{})

		td["path"] = req.URL.Path

		for k, v := range cfg.Items.Constant {
			td[k] = v
		}

		td["routes"] = cfg.Items.Routes

		for k, v := range cfg.Items.Query {
			var rows *sql.Rows
			if len(args) == 0 {
				log.Printf("%s\n", v)
				rows, err = db.Query(v)
			} else {
				log.Printf("%s, args: %v\n", v, args)
				rows, err = db.Query(v, args...)
			}
			if err != nil {
				log.Println(err)
				return
			}
			defer rows.Close()

			words := make([]string, 0, 50)
			for rows.Next() {
				var word string
				err = rows.Scan(&word)
				if err != nil {
					log.Println(err)
					return
				}
				words = append(words, word)
			}

			if k != "media" {
				td[k] = words
				continue
			}

			media := make([]map[string]string, 0, len(words))
			for _, word := range words {
				elem, err := getAllTags(db, word)
				if err != nil {
					log.Println(err)
					return
				}
				media = append(media, elem)
			}
			td[k] = media
		}

		tmpl, err := loadTemplate(cfg.Template)
		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, td)
		if err != nil {
			log.Fatal(err)
		}
	}
}

