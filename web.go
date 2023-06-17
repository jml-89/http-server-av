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
	Args     []string
	Redirect string
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

func revertTemplate(db *sql.DB, name string) error {
	// simplest and frankly disastrously crude
	// let's go!
	_, err := db.Exec(`
		delete from templates 
		where name is :name and rowid not in (
			select previous 
			from templates
		);`, sql.Named("name", name))
	return err
}

func addRoutes(db *sql.DB) ([]Route, error) {
	otherRoutes := Fastlinks //make([]Route, 0, 10)

	jsonRoutes, err := getTemplate(db, "routes")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	routes := make(map[string]Config)
	err = json.Unmarshal([]byte(jsonRoutes), &routes)
	if err != nil {
		log.Println("Failed to unmarshal routes JSON. Attempting to revert to previous version.")
		// bad route json? try to rollback to previous version
		err = revertTemplate(db, "routes")
		if err != nil {
			log.Println(err)
			return nil, err
		}
		return addRoutes(db)
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

		params := parseSearchTerms(req.Form["terms"])
		matches, err := lookup2(db, params)
		if err != nil {
			log.Fatal(err)
		}

		td["videos"] = matches
		td["count"] = len(matches)
		td["terms"] = strings.Join(req.Form["terms"], " ")
		td["routes"] = otherRoutes

		tmpl, err := loadTemplate(db, "index")
		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, td)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func loadTemplate(db *sql.DB, name string) (*template.Template, error) {
	rawBase, err := getTemplate(db, "base")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	rawTmpl, err := getTemplate(db, name)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	fns := make(map[string]any)
	// I've had mixed luck with html/template library escaping of UTF-8 strings for URL querystrings
	// Can be addressed by explicitly using the pertinent functions
	fns["escapequery"] = template.URLQueryEscaper
	fns["escapepath"] = url.PathEscape
	tmpl := template.Must(template.New("base").Parse(string(rawBase)))
	template.Must(tmpl.New("body").Funcs(fns).Parse(string(rawTmpl)))

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

		stmtArgs := make([]any, 0, len(cfg.Args))
		for k, vs := range req.Form {
			v := vs[0]
			stmtArgs = append(stmtArgs, sql.Named(k, v))
		}

		tx, err := db.Begin()
		if err != nil {
			log.Println(err)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(cfg.Query)
		defer stmt.Close()

		log.Printf("\t%s :: %v", cfg.Query, stmtArgs)
		_, err = stmt.Exec(stmtArgs...)
		if err != nil {
			log.Println(err)
			return
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
		for k, vs := range req.Form {
			v, err := url.QueryUnescape(vs[0])
			if err != nil {
				log.Println(err)
				return
			}
			//log.Printf("\t'%s':'%v'\n", k, v)
			args = append(args, sql.Named(k, v))
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

			cols, err := rows.Columns()
			if err != nil {
				log.Println(err)
				return
			}

			results := make([][]string, 0, 50)
			for rows.Next() {
				result := make([]any, len(cols), len(cols))
				for i := 0; i < len(cols); i++ {
					result[i] = new(string)
				}
				err = rows.Scan(result...)
				if err != nil {
					log.Println(err)
					return
				}
				r2 := make([]string, 0, len(cols))
				for _, field := range result {
					r2 = append(r2, *field.(*string))
				}
				results = append(results, r2)
			}

			// if there's just one field per row, flatten it into a simple 1 dimensional slice
			// no point incurring even more type diving overhead on the templates
			if len(cols) == 1 {
				flatsults := make([]string, 0, len(results))
				for _, result := range results {
					flatsults = append(flatsults, result[0])
				}
				td[k] = flatsults
			} else {
				td[k] = results
			}

		}

		tmpl, err := loadTemplate(db, cfg.Template)
		if err != nil {
			log.Fatal(err)
		}

		err = tmpl.Execute(w, td)
		if err != nil {
			log.Fatal(err)
		}
	}
}
