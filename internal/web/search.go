//Search handling
//Builds an SQL query out of the search parameters
//Not ideal, but works

package web

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
)

type SearchParameters struct {
	Vals    []string
	KeyVals map[string]string
}

// builds up a monster query (technical term) using a series of with statements
// that way the query can build in one direction, it's almost readable at the end
func (params *SearchParameters) Prepare() (string, []interface{}) {
	// the final query is made of bricks glued together
	// it mostly builds up a lot of subqueries
	// sqlite3 in-built function "instr" is used a lot
	bricks := make([]string, 0, 20)
	glue := "with"
	fills := make([]interface{}, 0, 20)

	paramCount := 0
	addParam := func(v interface{}) string {
		paramCount += 1
		name := fmt.Sprintf("searchparam%d", paramCount)
		fills = append(fills, sql.Named(name, v))
		return fmt.Sprintf(":%s", name)
	}

	searchCount := 0
	prevSearch := "tags"

	for _, v := range params.Vals {
		bricks = append(bricks, glue)
		glue = ","

		searchName := fmt.Sprintf("search%d", searchCount)
		bricks = append(bricks,
			fmt.Sprintf(`%s(filename, name, val, rowid) as (
				select filename, name, val, rowid
				from tags
				where filename in (
					select distinct(filename) 
					from %s
					where instr(lower(val), lower(%s)) > 0
				)
			)`, searchName, prevSearch, addParam(v)))
		searchCount += 1
		prevSearch = searchName
	}

	for k, v := range params.KeyVals {
		bricks = append(bricks, glue)
		glue = ","

		searchName := fmt.Sprintf("search%d", searchCount)
		bricks = append(bricks,
			fmt.Sprintf(`%s(filename, name, val, rowid) as (
				select filename, name, val, rowid
				from tags
				where filename in (
					select distinct(filename) 
					from %s
					where name is %s
					and instr(lower(val), lower(%s)) > 0
				)
			)`, searchName, prevSearch, addParam(k), addParam(v)))
		searchCount += 1
		prevSearch = searchName
	}

	bricks = append(bricks, fmt.Sprintf("select * from %s", prevSearch))

	return strings.Join(bricks, " "), fills
}

// This is unfortunately a SQL query string building function
// sqlite3 FSTS didn't really fit with my table design and goals
// FSTS could be used... but would need to create a specific FSTS table with data modified to suit it
func lookup2(db *sql.DB, params SearchParameters) ([]string, error) {
	res := make([]string, 0, 100)

	query, fills := params.Prepare()
	log.Printf("\n%s\n%v\n", query, fills)
	rows, err := db.Query(query, fills...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			return res, err
		}
		res = append(res, filename)
	}

	return res, err
}

// a search can be e.g. artist:devo title:'going under' remaster
// have to parse the key:value pairs
// as well as the pure value terms
func parseSearchTerms(formterms []string) SearchParameters {
	terms := make([]string, 0, 50)
	for _, term := range formterms {
		lo := 0
		i := 0

		basegrow := func(x int) {
			if i > lo {
				terms = append(terms, term[lo:i+x])
			}
			lo = i + 1
		}

		grow := func() {
			basegrow(0)
		}

		altgrow := func() {
			basegrow(1)
		}

		type roller func(rune) roller
		var quoteroller, baseroller roller

		baseroller = func(c rune) roller {
			switch c {
			case '"':
				grow()
				return quoteroller

			case ' ':
				grow()
				return baseroller

			case ':':
				altgrow()
				return baseroller

			default:
				return baseroller
			}
		}

		quoteroller = func(c rune) roller {
			switch c {
			case '"':
				grow()
				return baseroller

			default:
				return quoteroller
			}
		}

		// using a state machine, the work loop looks so clean now
		// but how obvious is the state machine code?
		step := baseroller
		for idx, c := range term {
			i = idx
			step = step(c)
		}
		i = len(term)
		grow()
	}

	params := SearchParameters{
		Vals:    make([]string, 0, 50),
		KeyVals: make(map[string]string),
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

	return params
}

