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
	"strconv"
	"strings"
)

type Route struct {
	Path   string
	Method string
	Alias  string
}

type Config struct {
	Get  *ConfigGet
	Post *ConfigPost
}

type TemplateFill struct {
	Routes   []Route
	Constant map[string]string
	Query    map[string]string
	Search   map[string]SearchBundle
}

type SearchBundle struct {
	Arg       string
	OrderBy   string
	OrderDesc bool
	Offset    int
	Limit     int
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

		stmt, err := tx.Prepare("select image from thumbnails where filename is ?;")
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

	rows, err := db.Query(`
		select 
			path
		from 
			routes;
	`)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		handler := createSuperSoftServe(db, path)

		log.Printf("Adding handler for '%s' route\n", path)
		http.HandleFunc(path, handler)
	}

	return otherRoutes, nil
}

/*
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
*/

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

func prepareSearchParams(form url.Values, bundle SearchBundle) SearchParameters {
	params := parseSearchTerms(form[bundle.Arg])

	params.Offset = bundle.Offset
	params.Limit = bundle.Limit
	params.OrderBy = bundle.OrderBy
	params.OrderDesc = bundle.OrderDesc

	if _, ok := form["offset"]; ok {
		n, err := strconv.Atoi(form["offset"][0])
		if err != nil {
			log.Println(err)
		} else {
			params.Offset = n
		}
	}

	if _, ok := form["limit"]; ok {
		n, err := strconv.Atoi(form["limit"][0])
		if err != nil {
			log.Println(err)
		} else {
			params.Limit = n
		}
	}

	if _, ok := form["orderby"]; ok {
		params.OrderBy = form["orderby"][0]
	}

	if _, ok := form["orderdesc"]; ok {
		params.OrderDesc = form["orderdesc"][0] == "true"
	}

	return params
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

		params := prepareSearchParams(form, bundle)

		query, pargs := params.Prepare()
		inserts[name] = query
		fills = pargs
	}

	return inserts, fills, err
}

func createSuperSoftServe(db *sql.DB, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		row := db.QueryRow(`
			select
				path,
				method,
				template
			from
				routes
			where
				path is :key;
		`, sql.Named("key", key))

		var path, method, templatename string
		err := row.Scan(&path, &method, &templatename)
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

		td := make(map[string]interface{})
		td["path"] = req.URL.Path
		td["routes"] = Fastlinks

		// if there's something in the search box, hold onto it
		if _, ok := req.Form["terms"]; ok {
			td["terms"] = strings.Join(req.Form["terms"], " ")
		}

		// search
		inserts, fills, err := getSearchGear(db, key, req.Form)
		if err != nil {
			log.Println(err)
			fmt.Fprintf(w, "%s", err)
			return
		}

		/*
			for k, v := range req.Form {
				log.Printf("%s: %v\n", k, v)
			}
			superSet := permutations(req.Form)
			for i, vs := range superSet {
				log.Printf("%d\n", i)
				for _, v := range vs {
					log.Printf("\t%s: %v\n", v.Name, v.Value)
				}
			}

			for k, v := range cfg.Items.Constant {
				td[k] = v
			}
		*/

		args := make([]any, 0, 10)
		for k, vs := range req.Form {
			v, err := url.QueryUnescape(vs[0])
			if err != nil {
				log.Println(err)
				fmt.Fprintf(w, "%s", err)
				return
			}
			//log.Printf("\t'%s':'%v'\n", k, v)
			args = append(args, sql.Named(k, v))
		}

		for _, fill := range fills {
			args = append(args, fill)
		}

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

		for rows.Next() {
			var name, query string
			err = rows.Scan(&name, &query)
			if err != nil {
				log.Println(err)
				fmt.Fprintf(w, "%s", err)
				return
			}

			for before, after := range inserts {
				query = strings.Replace(query, fmt.Sprintf("{{%s}}", before), after, -1)
			}
			fill := make([][]string, 0, 100)

			elems, err := runQuery(db, query, args)
			if err != nil {
				log.Println(err)
				fmt.Fprintf(w, "%s", err)
				return
			}
			fill = append(fill, elems...)

			maxlen := 0
			for _, xs := range fill {
				if len(xs) > maxlen {
					maxlen = len(xs)
				}
			}

			if maxlen == 1 {
				// flatten
				squash := make([]string, 0, len(fill))
				for _, xs := range fill {
					squash = append(squash, xs[0])
				}
				td[name] = squash
			} else {
				td[name] = fill
			}
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

//	"get": {
//		"template": "{{template_name}}",
//		"items": {
//			"constant": {
//				"{{variable_name}}": "{{some_value}}",
//				...
//				"{{variable_name}}": "{{some_value}}"
//			},
//			"query": {
//				"{{variable_name}}": "{{some_query}}",
//				...
//				"{{variable_name}}": "{{some_query}}"
//			}
//		}
//	}
//
// Warnings
//
//	constant and query values share the same namespace
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

		td := make(map[string]interface{})

		td["path"] = req.URL.Path
		td["routes"] = cfg.Items.Routes

		if _, ok := req.Form["terms"]; ok {
			td["terms"] = strings.Join(req.Form["terms"], " ")
		}

		inserts := make(map[string]string)

		var fills []interface{}

		for k, v := range cfg.Items.Search {
			params := parseSearchTerms(req.Form[v.Arg])

			params.Offset = v.Offset
			params.Limit = v.Limit
			params.OrderBy = v.OrderBy
			params.OrderDesc = v.OrderDesc

			if _, ok := req.Form["offset"]; ok {
				n, err := strconv.Atoi(req.Form["offset"][0])
				if err != nil {
					log.Println(err)
				} else {
					params.Offset = n
				}
			}

			if _, ok := req.Form["limit"]; ok {
				n, err := strconv.Atoi(req.Form["limit"][0])
				if err != nil {
					log.Println(err)
				} else {
					params.Limit = n
				}
			}

			if _, ok := req.Form["orderby"]; ok {
				params.OrderBy = req.Form["orderby"][0]
			}

			if _, ok := req.Form["orderdesc"]; ok {
				params.OrderDesc = req.Form["orderdesc"][0] == "true"
			}

			query, pargs := params.Prepare()
			inserts[k] = query
			fills = pargs
		}

		/*
			for k, v := range req.Form {
				log.Printf("%s: %v\n", k, v)
			}
			superSet := permutations(req.Form)
			for i, vs := range superSet {
				log.Printf("%d\n", i)
				for _, v := range vs {
					log.Printf("\t%s: %v\n", v.Name, v.Value)
				}
			}

			for k, v := range cfg.Items.Constant {
				td[k] = v
			}
		*/

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

		for _, fill := range fills {
			args = append(args, fill)
		}

		for k, query := range cfg.Items.Query {
			for before, after := range inserts {
				query = strings.Replace(query, fmt.Sprintf("{{%s}}", before), after, -1)
			}
			fill := make([][]string, 0, 100)

			elems, err := runQuery(db, query, args)
			if err != nil {
				log.Println(err)
				return
			}
			fill = append(fill, elems...)

			maxlen := 0
			for _, xs := range fill {
				if len(xs) > maxlen {
					maxlen = len(xs)
				}
			}

			if maxlen == 1 {
				// flatten
				squash := make([]string, 0, len(fill))
				for _, xs := range fill {
					squash = append(squash, xs[0])
				}
				td[k] = squash
			} else {
				td[k] = fill
			}
		}

		tmpl, err := loadTemplate(db, cfg.Template)
		if err != nil {
			log.Println(err)
			return
		}

		err = tmpl.Execute(w, td)
		if err != nil {
			log.Println(err)
			return
		}
	}
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

	/*
		// if there's just one field per row, flatten it into a simple 1 dimensional slice
		// no point incurring even more type diving overhead on the templates
		if len(cols) == 1 {
			flatsults := make([]string, 0, len(results))
			for _, result := range results {
				flatsults = append(flatsults, result[0])
			}
			return flatsults, nil
		}

		return results, nil
	*/
}
