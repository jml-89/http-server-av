package main

// TODO#1
// Streaming media
// TODO#2
// Javascript player

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
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

	pathDb := "info.db"

	done := make(chan bool)
	go func() {
		http.ListenAndServe(":8080", nil)
		done <- true
	}()

	log.Printf("Opening database %s\n", pathDb)
	db, err := sql.Open("sqlite3", pathDb)
	if err != nil {
		log.Fatalf("Failed to open %s: %s\n", pathDb, err)
	}
	defer db.Close()

	log.Printf("Initialising database...\n")
	err = initDB(db)
	if err != nil {
		log.Fatalf("Failed to initialise DB tables: %s\n", err)
	}

	http.Handle("/file/", http.StripPrefix("/file/", http.FileServer(http.Dir(pathMedia))))
	http.HandleFunc("/tmb/", serveThumbs(db))
	err = addRoutes(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Adding media files to database...\n")
	count, err := addFilesToDB(db, pathMedia)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v files added to database\n", count)

	log.Printf("Conducting word association...\n")
	err = wordassocs(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Fixing tags...\n")
	err = fixtags(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Culling missing files...\n")
	err = cullMissing(db, pathMedia)
	if err != nil {
		log.Fatal(err)
	}

	terminate := make(chan os.Signal)
	signal.Notify(terminate, os.Interrupt)

	select {
	case _ = <-done:
		log.Println("HTTP server terminated, quitting...")
	case _ = <-terminate:
		log.Println("SIGINT received, quitting...")
	}

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
