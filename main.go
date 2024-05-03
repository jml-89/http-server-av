package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/jml-89/httpfileserve/internal/av"
	"github.com/jml-89/httpfileserve/internal/web"
)

var flagPort = flag.Int("port", 8080, "webserver port")
var flagPath = flag.String("path", ".", "directory to serve")
var flagPathDB = flag.String("db", ".info.db", "media info database path")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()

	if *flagPath != "." {
		err := os.Chdir(*flagPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	pathMedia := "."
	pathDb := *flagPathDB

	done := make(chan bool)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", *flagPort), nil)
		if err != nil {
			log.Fatal(err)
		}
		done <- true
	}()

	log.Printf("Opening database %s\n", pathDb)
	db, err := sql.Open("sqlite3", pathDb)
	if err != nil {
		log.Fatalf("Failed to open %s: %s\n", pathDb, err)
	}
	defer db.Close()

	log.Printf("Initialising database: av side...\n")
	err = av.InitDB(db)
	if err != nil {
		log.Fatalf("Failed to initialise DB tables: %s\n", err)
	}

	log.Printf("Initialising database: web side...\n")
	err = web.InitDB(db)
	if err != nil {
		log.Fatalf("Failed to initialise DB tables: %s\n", err)
	}

	http.Handle("/file/", http.StripPrefix("/file/", http.FileServer(http.Dir(pathMedia))))
	http.HandleFunc("/tmb/", web.ServeThumbs(db))
	err = web.AddRoutes(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Adding media files to database...\n")
	count, err := av.AddFilesToDB(db, pathMedia)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v files added to database\n", count)

	terminate := make(chan os.Signal)
	signal.Notify(terminate, os.Interrupt)

	log.Printf("Initialisation complete, webserver running on port %d", *flagPort)

	select {
	case _ = <-done:
		log.Println("HTTP server terminated, quitting...")
	case _ = <-terminate:
		log.Println("SIGINT received, quitting...")
	}

	return
}
