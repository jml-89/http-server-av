package web

import (
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

func InitDB(db *sql.DB) error {
	log.Println("Initialising database")

	// WAL creates a few secondary files but generally I like it more
	// find it plays nicer in general with most systems
	// There's also WAL2 which addresses the ever-expanding write log problem
	// But I don't think WAL2 is a standard feature yet
	_, err := db.Exec("pragma journal_mode = wal;")
	if err != nil {
		log.Println(err)
		return err
	}

	err = DropAll(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initRoutes(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initRouteValues(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initTemplates(db)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("Database initialised")

	return nil
}

func DropAll(db *sql.DB) error {
	_, err := db.Exec(`
		drop table routes;
		drop table routevalues;
		drop table templatequeries;
		drop table templates;
		drop table templatesearches;
	`)
	return err
}

func initRoutesCore(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists routes (
			path text primary key,
			method text,
			alias text,
			template text,
			redirect text
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

	stmt, err := tx.Prepare(`
		insert or ignore into 
		routes (path, method, alias, template, redirect)
		values (:path, :method, :alias, :template, :redirect);
	`)

	for path, pack := range routeDefaults {
		_, err = stmt.Exec(
			sql.Named("path", path),
			sql.Named("method", pack["method"]),
			sql.Named("alias", pack["alias"]),
			sql.Named("template", pack["template"]),
			sql.Named("redirect", pack["redirect"]),
		)
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

func initRouteValues(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists routevalues (
			path text,
			k text,
			v text,
			primary key (path, k)
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

	stmt, err := tx.Prepare(`
		insert or ignore into 
		routevalues (path, k, v)
		values (:path, :k, :v);
	`)

	for path, pack := range routeDefaultValues {
		for k, v := range pack {
			_, err = stmt.Exec(
				sql.Named("path", path),
				sql.Named("k", k),
				sql.Named("v", v),
			)
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

	return nil
}

func initTemplateQueries(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists templatequeries (
			path text,
			name text,
			content text,
			primary key (path, name)
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

	stmt, err := tx.Prepare(`
		insert or ignore 
		into templatequeries (path, name, content)
		values (:path, :name, :content);
	`)
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmt.Close()

	for routename, pack := range routeDefaultQueries {
		for name, content := range pack {
			_, err = stmt.Exec(
				sql.Named("path", routename),
				sql.Named("name", name),
				sql.Named("content", content),
			)
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

	return nil
}

func initTemplateSearches(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists templatesearches (
			path text,
			name text,
			content text,
			primary key (path, name)
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

	stmt, err := tx.Prepare(`
		insert or ignore 
		into templatesearches (path, name, content)
		values (:path, :name, :content);
	`)
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmt.Close()

	for routename, pack := range routeDefaultSearches {
		for outname, bundle := range pack {
			b, err := json.Marshal(bundle)
			if err != nil {
				log.Println(err)
				return err
			}

			_, err = stmt.Exec(
				sql.Named("path", routename),
				sql.Named("name", outname),
				sql.Named("content", string(b)),
			)
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

	return nil
}

func initRoutes(db *sql.DB) error {
	err := initRoutesCore(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initTemplateQueries(db)
	if err != nil {
		log.Println(err)
		return err
	}

	err = initTemplateSearches(db)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func initTemplates(db *sql.DB) error {
	_, err := db.Exec(`
		create table if not exists templates (
			previous integer,
			name text,
			raw text,
			primary key (previous, name)
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

	stmtUpdate, err := tx.Prepare(`
		insert or ignore into 
		templates (name, raw, previous) 
		values    (   ?,   ?,        0);`)
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

