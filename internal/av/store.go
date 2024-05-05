package av

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strings"
	"path/filepath"
)

func InitDB(db *sql.DB) error {
	log.Println("Initialising database")

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	tables := []string{
		`create table if not exists tags (
			filename text,
			name text,
			val text,
			primary key (filename, name)
		);`,

		`create table if not exists thumbnails (
			filename text,
			image blob,
			primary key (filename)
		);`,

		`create table if not exists checked (
			filename text,
			primary key (filename)
		);`,
	}

	for _, table := range tables {
		_, err = tx.Exec(table)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// Only really necessary when checked table was introduced
	// Now everything gets added to checked correctly already
	// So this is redundant -- but no harm in keeping it around
	_, err = tx.Exec(
		`insert or ignore into checked (filename) 
			select filename 
			from tags 
			where name is 'diskfilename';
		`)
	if err != nil {
		log.Println(err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("Database initialised")

	return nil
}

// Adds thumbnail and metadata to database in a transaction
// You could consider updating more rows in a single transaction
// But how many rows at once? I do not know
// This has performed pretty reasonably in any case
// The limiting performance factor is elsewhere (handling media files)
func insertMedia(db *sql.DB, thumbnail []byte, metadata map[string]string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	stmtThumb, err := tx.Prepare(`
		insert or replace into 
			thumbnails (filename, image) 
			    values (       ?,     ?);
		`)
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtThumb.Close()

	stmtMetadata, err := tx.Prepare(`
		insert or replace into 
			  tags (filename, name, val) 
			values (       ?,    ?,   ?);
		`)
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtMetadata.Close()

	_, err = stmtThumb.Exec(metadata["thumbname"], thumbnail)
	if err != nil {
		log.Println(err)
		return err
	}

	for k, v := range metadata {
		_, err = stmtMetadata.Exec(metadata["diskfilename"], k, v)
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

// Majority of this function is orchestrating the goroutines
// There may be opportunity to expand some error handling
// However have not seen enough errors in testing to work on
func AddFilesToDB(db *sql.DB, path string) (int, error) {
	count := 0

	log.Printf("Adding files in %s to db\n", path)

	filenames, err := recls(path)
	if err != nil {
		return count, err
	}
	log.Printf("%v files found\n", len(filenames))

	filenames, err = filesNotInDB(db, filenames)
	if err != nil {
		return count, err
	}
	log.Printf("%v files have not been checked previously\n", len(filenames))

	if len(filenames) == 0 {
		log.Printf("Nothing to do here")
		return 0, nil
	}

	type Reply struct {
		filename  string
		stopped   bool
		thumbnail []byte
		metadata  map[string]string
		err       error
	}

	type Request struct {
		stop     bool
		filename string
		respond  chan<- Reply
	}

	replies := make(chan Reply)
	requests := make(chan Request)

	worker := func() {
		for {
			select {
			case req := <-requests:
				if req.stop {
					req.respond <- Reply{
						filename:  req.filename,
						stopped:   true,
						thumbnail: nil,
						metadata:  nil,
						err:       nil,
					}
					return
				}

				tmb, mt, err := parseMediaFile(req.filename)
				req.respond <- Reply{
					filename:  req.filename,
					stopped:   false,
					thumbnail: tmb,
					metadata:  mt,
					err:       err,
				}
			}
		}
	}

	workerCount := 4
	for i := 0; i < workerCount; i++ {
		go worker()
	}

	log.Printf("Scanning files with %d goroutines\n", workerCount)

	i := 0
	req := Request{
		stop:     false,
		filename: filenames[i],
		respond:  replies,
	}

	for workerCount > 0 {
		select {
		case reply := <-replies:
			if reply.stopped {
				workerCount--
				continue
			}

			_, err := db.Exec("insert into checked (filename) values (?);", reply.filename)
			if err != nil {
				break
			}

			if reply.err == errNotMediaFile {
				continue
			}

			if reply.err != nil {
				err = reply.err
				break
			}

			err = insertMedia(db, reply.thumbnail, reply.metadata)
			if err != nil {
				break
			}

			count++

		case requests <- req:
			i++
			if i < len(filenames) {
				req.filename = filenames[i]
			} else {
				req.filename = ""
				req.stop = true
			}
		}
	}

	log.Println("Files scanned, goroutines complete")


	log.Println("Word associations...")
	err = wordassocs(db)
	if err != nil {
		return count, err
	}

	log.Println("Fixing tags...")
	err = fixtags(db)
	if err != nil {
		return count, err
	}

	log.Println("Cull missing...")
	err = cullMissing(db, path)
	if err != nil {
		return count, err
	}


	return count, err
}

func cullMissing(db *sql.DB, dir string) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	stmtTagDel, err := tx.Prepare("delete from tags where filename is ?;")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtTagDel.Close()

	stmtAssocDel, err := tx.Prepare("delete from wordassocs where filename is ?;")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmtAssocDel.Close()

	count := 0
	rows, err := tx.Query(`
		select filename, val 
		from tags 
		where name is 'diskfilename';`)
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
			_, err = stmtTagDel.Exec(filename)
			if err != nil {
				log.Println(err)
				return err
			}

			_, err = stmtAssocDel.Exec(filename)
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

// returns a slice of files which are not in the database
func filesNotInDB(db *sql.DB, filenames []string) ([]string, error) {
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

// changes all tag names to lowercase
// album, ALBUM, Album -> album
// artist, ARTIST, Artist -> artist
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

var punctuation = " \r\n\t\"`~()[]{}<>&^%$#@?!+-=_,.:;|/\\*'"

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

// word associations used for related videos and search refinement features
// just takes tag contents, cleans them up, adds them to a key val table
func wordassocs(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists wordassocs (
			filename text,
			word text,
			primary key (filename, word) on conflict ignore
		);`)
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

	rows, err := db.Query(`
		select filename, val 
		from tags 
		where filename not in (
			select distinct(filename) 
			from wordassocs
		);`)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	stmtUpdate, err := tx.Prepare(`
		insert into 
			wordassocs (filename, word) 
			values     (       ?,    ?);
		`)
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

func recls(dir string) ([]string, error) {
	files := make([]string, 0, 128)

	var ls func(string) error
	ls = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				err = ls(path)
				if err != nil {
					return err
				}
				continue
			}
			files = append(files, path)
		}

		return nil
	}

	err := ls(dir)

	return files, err
}
