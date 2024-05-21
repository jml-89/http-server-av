package web

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"

	"github.com/jml-89/http-server-av/internal/util"
)

func getTemplate(db *sql.DB, name string) (string, error) {
	row := db.QueryRow(`
		select raw 
		from templates 
		where name is :name and not rowid in (
			select previous
			from templates
			where name is :name 
		);`, sql.Named("name", name))
	var raw string
	err := row.Scan(&raw)
	if err != nil {
		log.Printf("Could not find template '%s'\n", name)
		return raw, err
	}

	return raw, nil
}

func getRouteVals(db *sql.DB, key string) (map[string]string, error) {
	rows, err := db.Query(`
		select k, v
		from routevalues
		where path is :key;
		`, sql.Named("key", key))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	kvs := make(map[string]string)
	for rows.Next() {
		var k, v string
		err = rows.Scan(&k, &v)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		kvs[k] = v
	}

	return kvs, err
}

func runTemplateQueries(db *sql.DB, key string, inserts map[string]string, args []any) (map[string][][]string, error) {
	names, queries, err := util.AllRows2[string, string](db, `
		select name, content
		from templatequeries
		where path is :key;`, sql.Named("key", key))
	if err != nil {
		log.Println(err)
		return nil, err
	}

	queryResults := make(map[string][][]string)
	for i := 0; i < len(names); i++ {
		name := names[i]
		query := queries[i]

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
			return nil, err
		}

		queryResults[name] = elems
	}

	return queryResults, err
}

func runQuery(db *sql.DB, query string, args []any) ([][]string, error) {
	var rows *sql.Rows
	var err error
	if len(args) == 0 {
		rows, err = db.Query(query)
	} else {
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
