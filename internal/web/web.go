// HTTP request handling
// Most of this is tying up the route and html template information
package web

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

	"github.com/jml-89/http-server-av/internal/util"
)

type Route struct {
	Path  string
	Alias string
}

type TemplateFill struct {
	Routes []Route
	Query  map[string]string
	Search map[string]SearchBundle
}

type SearchBundle struct {
	Arg string
}

// Thumbnails act like a simple fileserver
// But they're served out of the database (stored as blobs)
func ServeThumbs(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		thumbServer(db, w, r)
	}
}

// Adds routes to http default handler (global...)
// Routes are stored in the database too
// Everything is in the database...
func AddRoutes(db *sql.DB) error {
	paths, err := util.AllRows1(db, `select path from routes;`, "")
	if err != nil {
		log.Println(err)
		return err
	}

	for _, path := range paths {
		handler := createSuperSoftServe(db, path)
		http.HandleFunc(path, handler)
	}

	return nil
}

func thumbServer(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	thumbname := r.URL.Path[5:]

	var blob []byte
	err := db.QueryRow(`
		select image 
		from thumbnail
		where thumbname is :thumbname;
	`, sql.Named("thumbname", thumbname)).Scan(&blob)
	if err != nil {
		log.Printf(
			"Wanted thumbnail '%s', got: %v",
			thumbname,
			err,
		)
		return
	}

	_, err = w.Write(blob)
	if err != nil {
		log.Println(err)
		return
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
	fns["prettyprint"] = func(s string) string {
		s = strings.Map(func(r rune) rune {
			switch r {
			case '-':
				return ' '
			case '_':
				return ' '
			}
			return r
		}, s)
		parts := strings.Split(s, ".")
		s = strings.Join(parts[:len(parts)-1], ".")
		return s
	}
	tmpl := template.Must(template.New("base").Parse(string(rawBase)))
	template.Must(tmpl.New("body").Funcs(fns).Parse(string(rawTmpl)))

	return tmpl, nil
}

func getSearchGear(db *sql.DB, key string, form url.Values) (map[string]string, []interface{}, error) {
	inserts := make(map[string]string)
	var fills []interface{}

	rows, err := db.Query(`
		select name, content
		from templatesearches
		where path is :key;
	`, sql.Named("key", key))
	if err != nil {
		log.Println(err)
		return inserts, fills, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, content string
		err = rows.Scan(&name, &content)
		if err != nil {
			log.Println(err)
			return inserts, fills, err
		}

		var bundle SearchBundle
		err = json.Unmarshal([]byte(content), &bundle)
		if err != nil {
			log.Println(err)
			return inserts, fills, err
		}

		params := parseSearchTerms(form[bundle.Arg])
		query, pargs := params.Prepare()
		inserts[name] = query
		fills = pargs
	}

	return inserts, fills, err
}

func getFastLinks(db *sql.DB) ([]Route, error) {
	rows, err := db.Query(`
		select
			alias,
			path
		from
			routes
		where
			alias is not null
		`)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	routes := make([]Route, 0, 10)
	for rows.Next() {
		var alias, path string

		err := rows.Scan(&alias, &path)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		routes = append(routes, Route{Alias: alias, Path: path})
	}

	return routes, nil
}

func getTags(db *sql.DB, filename string) (map[string]string, error) {
	rows, err := db.Query(
		"select name, val from tags where filename is :filename",
		sql.Named("filename", filename))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	xs := make(map[string]string)
	for rows.Next() {
		var n, v string
		err = rows.Scan(&n, &v)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		xs[n] = v
	}

	return xs, err
}

func createSuperSoftServe(db *sql.DB, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		superSoftServe(db, key, w, req)
	}
}

// Nothing saved, everything pulled from the database every time
// Good for development & testing phase
func superSoftServe(db *sql.DB, key string, w http.ResponseWriter, req *http.Request) {
	row := db.QueryRow(`
		select
			path,
			method,
			template,
			redirect
		from
			routes
		where
			path is :key;
		`, sql.Named("key", key))

	var path, method, templatename, redirect string
	err := row.Scan(&path, &method, &templatename, &redirect)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	if strings.ToUpper(req.Method) != strings.ToUpper(method) {
		fmt.Fprintf(w, "Expected %s, got %s", method, req.Method)
		return
	}

	err = req.ParseForm()
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	// default values when not present in form
	kvs, err := getRouteVals(db, key)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}
	for k, v := range kvs {
		if _, ok := req.Form[k]; ok {
			continue
		}
		req.Form[k] = []string{v}
	}

	// search
	inserts, fills, err := getSearchGear(db, key, req.Form)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	td := make(map[string]interface{})
	td["path"] = req.URL.Path
	td["routes"], err = getFastLinks(db)

	// only the first value for each key is taken
	args := make([]any, 0, 10)
	for k, vs := range req.Form {
		v := vs[0]
		if req.Method == "GET" {
			v, err = url.QueryUnescape(vs[0])
			if err != nil {
				log.Println(err)
				fmt.Fprintf(w, "%s", err)
				return
			}
		}

		args = append(args, sql.Named(k, v))
		td[k] = v
	}
	args = append(args, fills...)

	if _, ok := req.Form["filename"]; ok {
		s, err := url.QueryUnescape(req.Form["filename"][0])
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		tags, err := getTags(db, s)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		for k, v := range tags {
			td[k] = v
		}
	}

	queryResults, err := runTemplateQueries(db, key, inserts, args)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	// results by default returns an array of string arrays
	// each result row is an array representing each column
	// however many queries only seek one column
	// for those queries, flatted into an array of strings
	// simplifies operations in the template
	// which is already overloaded with complexity otherwise
	for name, results := range queryResults {
		maxlen := 0
		for _, xs := range results {
			if len(xs) > maxlen {
				maxlen = len(xs)
			}
		}

		if maxlen == 1 && len(results) == 1 {
			td[name] = results[0][0]
		} else if maxlen == 1 {
			squash := make([]string, 0, len(results))
			for _, xs := range results {
				squash = append(squash, xs[0])
			}
			td[name] = squash
		} else {
			td[name] = results
		}
	}

	if templatename == "" && redirect == "" {
		redirect = req.URL.Path
	}

	if redirect != "" {
		http.Redirect(w, req, redirect, http.StatusFound)
		return
	}

	tmpl, err := loadTemplate(db, templatename)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	err = tmpl.Execute(w, td)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}
}

func emptyHandler(w http.ResponseWriter, req *http.Request) {
	// nothing! wahey
}
