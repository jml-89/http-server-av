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

type TemplateFill struct {
	Routes []Route
	//Constant map[string]string
	Query  map[string]string
	Search map[string]SearchBundle
}

type SearchBundle struct {
	Arg string
}

func serveThumbs(db *sql.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		thumbServer(db, w, r)
	}
}

func thumbServer(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[5:]

	var blob []byte
	err := db.QueryRow(`
		select image 
		from thumbnails 
		where filename is :filename;
	`, sql.Named("filename", filename)).Scan(&blob)
	if err != nil {
		log.Printf(
			"Wanted thumbnail for '%s', got: %v",
			filename,
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

func addRoutes(db *sql.DB) error {
	rows, err := db.Query(`select path from routes;`)
	if err != nil {
		log.Println(err)
		return err
	}

	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			log.Println(err)
			return err
		}

		handler := createSuperSoftServe(db, path)

		log.Printf("Adding handler for '%s' route\n", path)
		http.HandleFunc(path, handler)
	}

	return nil
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

func createSuperSoftServe(db *sql.DB, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		superSoftServe(db, key, w, req)
	}
}

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

	// search
	inserts, fills, err := getSearchGear(db, key, req.Form)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	// for the time being only the first value for each key is taken
	args := make([]any, 0, 10)
	for k, vs := range req.Form {
		v, err := url.QueryUnescape(vs[0])
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		args = append(args, sql.Named(k, v))
	}
	args = append(args, fills...)

	rows, err := db.Query(`
		select name, content
		from templatequeries
		where path is :key;
		`, sql.Named("key", key))
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}
	defer rows.Close()

	queryResults := make(map[string][][]string)
	for rows.Next() {
		var name, query string
		err = rows.Scan(&name, &query)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		for before, after := range inserts {
			query = strings.Replace(
				query,
				fmt.Sprintf("{{%s}}", before), after,
				-1,
			)
		}

		elems, err := runQuery(db, query, args)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		queryResults[name] = elems
	}

	if templatename == "" && redirect == "" {
		redirect = req.URL.Path
	}
	if redirect != "" {
		http.Redirect(w, req, redirect, http.StatusFound)
	}

	tmpl, err := loadTemplate(db, templatename)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "%s", err)
		return
	}

	td := make(map[string]interface{})
	td["path"] = req.URL.Path
	td["routes"] = Fastlinks

	// if there's something in the search box, hold onto it
	//if _, ok := req.Form["terms"]; ok {
	//	td["terms"] = strings.Join(req.Form["terms"], " ")
	//}

	// search terms, sort order, page number, and so on
	for k, vs := range req.Form {
		td[k] = strings.Join(vs, " ")
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

		if maxlen == 1 {
			squash := make([]string, 0, len(results))
			for _, xs := range results {
				squash = append(squash, xs[0])
			}
			td[name] = squash
		} else {
			td[name] = results
		}
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

func permutations(form map[string][]string) [][]sql.NamedArg {
	link := make([][]sql.NamedArg, 0, len(form))
	for k, vs := range form {
		linkline := make([]sql.NamedArg, 0, len(vs))
		for _, v := range vs {
			s, err := url.QueryUnescape(v)
			if err != nil {
				s = v
			}
			linkline = append(linkline, sql.Named(k, s))
		}
		link = append(link, linkline)
	}

	/*
		for _, vs := range link {
			for _, v := range vs {
				log.Printf("\t%s: %v\n", v.Name, v.Value)
			}
		}
	*/

	var step func([][]sql.NamedArg) [][]sql.NamedArg
	step = func(form [][]sql.NamedArg) [][]sql.NamedArg {
		res := make([][]sql.NamedArg, 0, 10)
		if len(form) < 1 {
			return res
		}

		// last entry
		if len(form) == 1 {
			for _, x := range form[0] {
				res = append(res, []sql.NamedArg{x})
			}
			return res
		}

		for _, x := range form[0] {
			for _, ys := range step(form[1:]) {
				zs := make([]sql.NamedArg, 0, len(form))
				zs = append(zs, x)
				zs = append(zs, ys...)
				res = append(res, zs)
			}
		}

		return res
	}

	return step(link)
}

func runQuery(db *sql.DB, query string, args []any) ([][]string, error) {
	var rows *sql.Rows
	var err error
	if len(args) == 0 {
		log.Printf("%s\n", query)
		rows, err = db.Query(query)
	} else {
		log.Printf("Query: %s\n", query)
		for _, arg := range args {
			log.Printf("\t%v\n", arg)
		}
		rows, err = db.Query(query, args...)
	}
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		log.Println(err)
		return nil, err
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
			return nil, err
		}

		r2 := make([]string, 0, len(cols))
		for _, field := range result {
			r2 = append(r2, *field.(*string))
		}

		results = append(results, r2)
	}

	return results, nil
}
