package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strings"
)

// DESIGN#1
// "Search" is just by tags with a rather badly constructed search query builder
// It may be wise to migrate to a schema that uses the SQLite FTS5 (Full Text Search) extension
// It handles the search term language on its own
// However it is more difficult to search by tag given the table layout I am using

func initDB(db *sql.DB) error {
	log.Println("Initialising database")

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS tags (filename text, name text, val text, PRIMARY KEY(filename, name));")
	if err != nil {
		log.Println(err)
		return err
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS thumbnails (filename text, image blob, PRIMARY KEY(filename));")
	if err != nil {
		log.Println(err)
		return err
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS checked (filename text, PRIMARY KEY(filename));")
	if err != nil {
		log.Println(err)
		return err
	}

	// Only really necessary when checked table was introduced
	// Now everything gets added to checked correctly already
	// So this is redundant
	_, err = tx.Exec("insert or ignore into checked (filename) select filename from tags where name is 'diskfilename';")
	if err != nil {
		log.Println(err)
		return err
	}


	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	err = initTemplates(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initRest(db)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("Database initialised")

	return nil
}

func initRest(db *sql.DB) error {
	raw, err := getTemplate(db, "init")
	if err != nil {
		log.Println(err)
		return err
	}

	var statements []string
	err = json.Unmarshal([]byte(raw), &statements)
	if err != nil {
		log.Println(err)
		return err
	}

	for _, statement := range statements {
		log.Println(statement)
		_, err = db.Exec(statement)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func addFilesToDB(db *sql.DB, path string) (int, error) {
	count := 0

	log.Printf("Adding files in %s to db\n", path)

	filenames, err := recls(path)
	if err != nil {
		return count, err
	}
	log.Printf("%v candidates found\n", len(filenames))

	filenames, err = differenceFilesDB(db, filenames)
	if err != nil {
		return count, err
	}
	log.Printf("%v candidates valid\n", len(filenames))

	for _, filename := range filenames {
		_, err = db.Exec("insert into checked (filename) values (?);", filename)
		if err != nil {
			continue
		}

		// Assume any failed thumbnail gen means it wasn't a media file
		// what about audio? audio is skipped
		thumbnail, err := CreateThumbnail(filename)
		if err != nil {
			continue
		}

		metadata, err := GetMetadata(filename)
		if err != nil {
			return count, err
		}

		info, err := os.Stat(filename)
		if err != nil {
			return count, err
		}

		metadata["diskfiletime"] = info.ModTime().UTC().Format("2006-01-02T15:04:05")
		metadata["diskfilename"] = filename
		thumbname := fmt.Sprintf("%s.webp", filename)
		metadata["thumbname"] = thumbname

		tx, err := db.Begin()
		if err != nil {
			return count, err
		}
		defer tx.Rollback()

		stmtThumb, err := tx.Prepare("INSERT OR REPLACE INTO thumbnails (filename, image) VALUES (?, ?);")
		if err != nil {
			return count, err
		}
		defer stmtThumb.Close()

		stmtMetadata, err := tx.Prepare("INSERT OR REPLACE INTO tags (filename, name, val) VALUES (?, ?, ?);")
		if err != nil {
			return count, err
		}
		defer stmtMetadata.Close()

		_, err = stmtThumb.Exec(thumbname, thumbnail)
		if err != nil {
			return count, err
		}

		for k, v := range metadata {
			_, err = stmtMetadata.Exec(filename, k, v)
			if err != nil {
				log.Println(err)
				return count, err
			}
		}

		err = tx.Commit()
		if err != nil {
			log.Println(err)
			return count, err
		}

		count++

	}

	return count, nil
}

func initTemplates(db *sql.DB) error {
	_, err := db.Exec("create table if not exists templates (previous integer, name text, raw text, primary key (previous, name));")
	if err != nil {
		log.Println(err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	stmtUpdate, err := tx.Prepare("insert or ignore into templates (name, raw, previous) values (?, ?, 0);")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtUpdate.Close()

	for name, raw := range starterTemplates {
		_, err = stmtUpdate.Exec(name, raw)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

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
		log.Println(err)
		return raw, err
	}

	return raw, nil
}

func cullMissing(db *sql.DB, dir string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("delete from tags where filename is ?;")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmt.Close()

	count := 0
	rows, err := tx.Query("select filename, val from tags where name is 'diskfilename';")
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var filename string
		var path string
		err = rows.Scan(&filename, &path)
		if err != nil {
			log.Println(err)
			return err
		}

		_, err = os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			// file moved, or removed
			log.Printf("Removing %s\n", path)
			_, err = stmt.Exec(filename)
			if err != nil {
				log.Println(err)
				return err
			}
			count += 1
			continue
		}

		if err != nil {
			log.Println(err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("%d missing media items removed\n", count)
	return nil
}

func differenceFilesDB(db *sql.DB, filenames []string) ([]string, error) {
	newFiles := make([]string, 0, len(filenames))

	rows, err := db.Query("select filename from checked;")
	if err != nil {
		log.Println(err)
		return newFiles, err
	}
	defer rows.Close()

	dbnames := make(map[string]bool)
	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			log.Println(err)
			return newFiles, err
		}

		dbnames[filename] = true
	}

	for _, filename := range filenames {
		if _, ok := dbnames[filename]; !ok {
			newFiles = append(newFiles, filename)
		}
	}

	return newFiles, nil
}

func updateDB(db *sql.DB, dir string) error {
	files, err := recls(dir)
	if err != nil {
		return err
	}

	files, err = differenceFilesDB(db, files)

	for _, file := range files {
		log.Printf("%s not in database\n", file)
	}

	return nil
}

func getAllTags(db *sql.DB, filename string) (map[string]string, error) {
	res := make(map[string]string)

	stmt := "select name, val from tags where filename is ?;"
	rows, err := db.Query(stmt, filename)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var n, v string
		err = rows.Scan(&n, &v)
		if err != nil {
			return res, err
		}
		res[n] = v
		count++
	}

	res["filename"] = filename
	//res["filenamequery"] = url.QueryEscape(filename)
	//res["filenamepath"] = url.PathEscape(filename)
	//res["thumbname"] = url.PathEscape(res["thumbname"])

	return res, nil
}

type SearchParameters struct {
	Vals        []string
	KeyVals     map[string]string
	Limit       int
	RandomOrder bool
}

func lookup2(db *sql.DB, params SearchParameters) ([][]string, error) {
	rescap := params.Limit
	if rescap == 0 {
		rescap = 50
	}

	res := make([][]string, 0, rescap)

	bricks := make([]string, 0, 20)
	bricks = append(bricks, "select filename, val from tags where name is 'thumbname'")
	glue := "and"
	fills := make([]interface{}, 0, 20)

	if len(params.Vals) > 0 {
		for _, v := range params.Vals {
			bricks = append(bricks, glue)
			//bricks = append(bricks, "filename in (select distinct(filename) from tags where val like '%' || ? || '%')")
			bricks = append(bricks, "filename in (select distinct(filename) from tags where INSTR(LOWER(val), LOWER(?)) > 0)")
			fills = append(fills, v)
			glue = "and"
		}
	}

	if len(params.KeyVals) > 0 {
		for k, v := range params.KeyVals {
			bricks = append(bricks, glue)
			//bricks = append(bricks, "filename in (select filename from tags where name is ? and val like '%' || ? || '%')")
			bricks = append(bricks, "filename in (select filename from tags where name is ? and INSTR(LOWER(val), LOWER(?)) > 0)")
			fills = append(fills, k, v)
			glue = "and"
		}
	}

	if params.RandomOrder {
		bricks = append(bricks, "order by random()")
	}

	if params.Limit > 0 {
		bricks = append(bricks, "limit ?")
		fills = append(fills, params.Limit)
	}

	bricks = append(bricks, ";")

	query := strings.Join(bricks, " ")
	log.Printf("\n%s\n%v\n", query, fills)
	rows, err := db.Query(query, fills...)
	if err != nil {
		return res, err
	}

	for rows.Next() {
		cols := make([]string, 0, 2)

		var filename string
		var thumbname string
		err = rows.Scan(&filename, &thumbname)
		if err != nil {
			return res, err
		}
		cols = append(cols, filename, thumbname)
		res = append(res, cols)
	}

	return res, err
}

func fixtags(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	upd, err := tx.Prepare("update tags set name = ? where name is ?;")
	if err != nil {
		log.Println(err)
		return err
	}
	defer upd.Close()

	rows, err := tx.Query("select distinct(name) from tags;")
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			log.Println(err)
			return err
		}

		lower := strings.ToLower(name)
		if lower == name {
			continue
		}

		_, err = upd.Exec(lower, name)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

var punctuation = " \r\n\t\"`~()[]{}<>&^%$#@?!+-=_,.:;|/\\*"

func stringsplat(s, cutset string) []string {
	res := make([]string, 0, 100)

	var b strings.Builder
	for _, c := range s {
		if strings.ContainsRune(cutset, c) {
			if b.Len() > 0 {
				res = append(res, b.String())
				b.Reset()
			}
		} else {
			b.WriteRune(c)
		}
	}

	if b.Len() > 0 {
		res = append(res, b.String())
		b.Reset()
	}

	return res
}

func wordassocs(db *sql.DB) error {
	_, err := db.Exec("create table if not exists wordassocs (filename text, word text, primary key(filename, word) on conflict ignore);")
	if err != nil {
		log.Println(err)
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	rows, err := db.Query("select filename, val from tags where filename not in (select distinct(filename) from wordassocs);")
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	stmtUpdate, err := tx.Prepare("insert into wordassocs (filename, word) values (?, ?);")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtUpdate.Close()

	for rows.Next() {
		var filename string
		var words string
		err = rows.Scan(&filename, &words)
		if err != nil {
			log.Println(err)
			return err
		}

		for _, word := range stringsplat(words, punctuation) {
			word = strings.ToLower(word)

			_, err = stmtUpdate.Exec(filename, word)
			if err != nil {
				log.Println(err)
				return err
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	return err
}

func parseSearchTerms(formterms []string) SearchParameters {
	terms := make([]string, 0, 50)
	for _, term := range formterms {
		inQuotes := false
		lo := 0
		i := 0
		for _, c := range term {
			if inQuotes {
				if c == '"' {
					if i > lo {
						terms = append(terms, term[lo:i])
					}
					lo = i + 1
					inQuotes = false
				}
			} else if c == '"' {
				if i > lo {
					terms = append(terms, term[lo:i])
				}
				inQuotes = true
				lo = i + 1
			} else if c == ' ' {
				if i > lo {
					terms = append(terms, term[lo:i])
				}
				lo = i + 1
			} else if c == ':' {
				if i > lo {
					terms = append(terms, term[lo:i+1])
				}
				lo = i + 1
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

	return params
}
