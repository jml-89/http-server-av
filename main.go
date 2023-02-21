package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var usage = "servemedia [path]"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if len(os.Args) > 1 {
		err := os.Chdir(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
	}

	pathMedia, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	pathMedia = "."

	//db.SetMaxOpenConn(1)
	pathDb := "file:info.db?_journal_mode=WAL&mode=rwc"

	db, err := sql.Open("sqlite3", pathDb)
	if err != nil {
		log.Fatalf("Failed to open %s: %s\n", pathDb, err)
	}
	defer db.Close()

	err = initDB(db)
	if err != nil {
		log.Fatalf("Failed to initialise DB tables: %s\n", err)
	}

	count, err := addFilesToDB(db, pathMedia)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v files added to database\n", count)

	err = wordcount(db)
	if err != nil {
		log.Fatal(err)
	}

	err = fixtags(db)
	if err != nil {
		log.Fatal(err)
	}

	err = cullMissing(db, pathMedia)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/file/", http.StripPrefix("/file/", http.FileServer(http.Dir(pathMedia))))
	//thumbnails will be served out of database instead of from filesystem
	//http.HandleFunc("/watch/", serveVideo(db))
	http.HandleFunc("/tmb/", serveThumbs(db))
	rs, err := addRoutes(db)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/search", createSearchHandler(db, rs))
	log.Fatal(http.ListenAndServe(":8080", nil))

	return
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

