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

	"math"

	"github.com/jml-89/http-server-av/internal/av"
	"github.com/jml-89/http-server-av/internal/web"
)

var flagPort = flag.Int("port", 8080, "webserver port")
var flagPath = flag.String("path", ".", "directory to serve")
var flagPathDB = flag.String("db", ".info.db", "media info database path")
var flagConc = flag.Int("conc", 2, "number of concurrent file scanner / thumbnailers to run")

func mySqrt(x int) int {
	return int(math.Sqrt(float64(x)))
}

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

	// Dude, just use
	//   go build -tags sqlite_math_functions
	// Was NOT working for me
	// This is a super cool feature though
	// So easy to connect a Go func to sqlite3!
	sql.Register("sqlite3_with_square_root", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			return conn.RegisterFunc("sqrt", mySqrt, true)
		},
	})

	log.Printf("Opening database %s\n", pathDb)
	db, err := sql.Open("sqlite3_with_square_root", pathDb)
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

	terminate := make(chan os.Signal)
	signal.Notify(terminate, os.Interrupt)

	log.Printf("Initialisation complete, webserver running on port %d", *flagPort)

	go func() {
		for {
			_, err = av.AddFilesToDB(db, *flagConc, 2, pathMedia)
			if err != nil {
				log.Println(err)
				return
			}

			time.Sleep(time.Duration(rand.Intn(10)) * time.Second)
		}
	}()

	go func() {
		ev, err := av.NewEvaluator(*flagConc)
		if err != nil {
			log.Println(err)
			return
		}

		for {
			_, err = ev.Run(db)
			if err != nil {
				log.Println(err)
				return
			}

			numImproved, err := av.Improver(db)
			if err != nil {
				log.Println(err)
				return
			}

			if numImproved == 0 {
				time.Sleep(time.Duration(rand.Intn(60)) * time.Second)
			}
		}
	}()

	select {
	case _ = <-done:
		log.Println("HTTP server terminated, quitting...")
	case _ = <-terminate:
		log.Println("SIGINT received, quitting...")
	}

	return
}
