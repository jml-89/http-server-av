package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"log"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/jml-89/http-server-av/internal/av"
	"github.com/jml-89/http-server-av/internal/util"
	"github.com/jml-89/http-server-av/internal/web"
)

var flagPort = flag.Int("port", 8080, "webserver port")
var flagPath = flag.String("path", ".", "directory to serve")
var flagPathDB = flag.String("db", ".info.db", "media info database path")
var flagConc = flag.Int("conc", 2, "number of concurrent file scanner / thumbnailers to run")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("http-server-av initialising")

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

	// Dude, just use
	//   go build -tags sqlite_math_functions
	// Was NOT working for me
	// This is a super cool feature though
	// So easy to connect a Go func to sqlite3!
	// and can use it to attach a few other functions too
	sql.Register("sqlite3_custom", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			err := conn.RegisterFunc("sqrt", util.MySqrt, true)
			if err != nil {
				return err
			}

			err = conn.RegisterFunc("scorefn", av.ScoreFunc, true)
			if err != nil {
				return err
			}

			return nil
		},
	})

	db, err := sql.Open("sqlite3_custom", pathDb)
	if err != nil {
		log.Fatalf("Failed to open %s: %s\n", pathDb, err)
	}
	defer db.Close()

	//db.SetMaxOpenConns(1)

	// WAL creates a few secondary files but generally I like it more
	// Allows more concurrent operations which is kind of critical in a Go runtime environment
	//
	// There's also WAL2 which addresses the ever-expanding write log problem
	// But it *still* isn't on the main branch
	_, err = db.Exec("pragma journal_mode = wal;")
	if err != nil {
		log.Fatalf("Failed to set journal_mode to wal (??): %s", err)
	}

	err = av.InitDB(db)
	if err != nil {
		log.Fatalf("Failed to initialise DB tables: %s\n", err)
	}

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

	terminate := make(chan os.Signal)
	signal.Notify(terminate, os.Interrupt)

	fmt.Printf("*\n*\tWebserver running on port %d\n*\n", *flagPort)

	go func() {
		ignores := []string{pathDb}
		_, err := av.AddFilesToDB(db, ignores, *flagConc, pathMedia)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Initial media scan complete")
		log.Println("Starting thumbnail improver")

		go thumbImprover(db, *flagConc)

		for {
			n, err := av.AddFilesToDB(db, ignores, *flagConc, pathMedia)
			if err != nil {
				log.Println(err)
				return
			}

			if n > 0 {
				_, err = db.Exec("pragma wal_checkpoint(TRUNCATE);")
				if err != nil {
					log.Println(err)
					if err.Error() == "database is locked" {
						err = nil
					} else {
						return
					}
				}
			}

			time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
		}
	}()

	select {
	case _ = <-done:
		log.Println("HTTP server terminated, quitting...")
	case _ = <-terminate:
		log.Println("SIGINT received, quitting...")
	}

	_, err = db.Exec("pragma analyze_limit = 400;")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("pragma optimize;")
	if err != nil {
		log.Fatal(err)
	}

	return
}

func thumbImprover(db *sql.DB, numThreads int) {
	ev, err := av.NewEvaluator(numThreads)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		_, err = ev.Run(db)
		if err != nil {
			log.Println(err)
			if err.Error() == "database is locked" {
				err = nil
				time.Sleep(time.Duration(rand.Intn(30)) * time.Second)
			} else {
				return
			}
		}

		numImproved, err := av.Improver(db)
		if err != nil {
			log.Println(err)
			if err.Error() == "database is locked" {
				err = nil
				time.Sleep(time.Duration(rand.Intn(30)) * time.Second)
			} else {
				return
			}
		}

		if numImproved == 0 {
			time.Sleep(time.Duration(rand.Intn(60)) * time.Second)
			continue
		}

		_, err = db.Exec("pragma wal_checkpoint(TRUNCATE);")
		if err != nil {
			log.Println(err)
			if err.Error() == "database is locked" {
				err = nil
			} else {
				return
			}
		}
	}
}
