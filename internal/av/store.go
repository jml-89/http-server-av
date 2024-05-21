//Database-centric routines
// Two main functions: InitDB, and AddFilesToDB

// Initialise database tables
// Adding media file information to the database, including thumbnails and metadata
// Filling in word associations table
// Removing database entries for files that are no longer on disk

package av

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jml-89/http-server-av/internal/util"
)

func InitDB(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	tables := []string{
		`create table if not exists filestat (
			filename text,
			filesize integer not null,
			primary key (filename)
		);`,

		`create table if not exists mediastat (
			filename text,
			canseek integer not null,
			probes integer not null,
			facechecked integer not null,
			bestthumb text not null,
			bestscore real not null,
			primary key (filename)
		);`,

		`create table if not exists tags (
			filename text,
			name text,
			val text not null,
			primary key (filename, name)
		);`,

		`create index if not exists tags_filename_idx on tags(filename);`,
		`create index if not exists tags_name_idx on tags(name);`,

		`create table if not exists thumbmap (
			filename text,
			thumbname text,
			primary key (filename, thumbname)
		);`,

		`create index if not exists thumbmap_filename_idx on thumbmap(filename);`,
		`create index if not exists thumbmap_thumbname_idx on thumbmap(thumbname);`,

		`create table if not exists thumbface (
			thumbname text not null,
			area integer not null,
			confidence real not null,
			quality real not null
		);`,

		`create index if not exists thumbface_thumbname_idx on thumbface(thumbname);`,

		`create table if not exists wordassocs (
			filename text,
			word text,
			primary key (filename, word) on conflict ignore
		);`,

		`create index if not exists wordassocs_filename_idx on wordassocs(filename);`,
		`create index if not exists wordassocs_word_idx on wordassocs(word);`,

		`create table if not exists thumbnail (
			thumbname string,
			image blob not null,
			facechecked integer not null,
			area integer not null,
			confidence real not null,
			quality real not null,
			score real not null,
			primary key (thumbname)
		);`,
	}

	for _, table := range tables {
		_, err = tx.Exec(table)
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

	err = RescoreAll(db)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func insertThumbnail(tx *sql.Tx, filename string, thumbnail Thumbnail) error {
	thumbName := fmt.Sprintf("%s.webp", thumbnail.digest)
	_, err := tx.Exec(`
		insert or replace into 
			thumbnail (thumbname, image, facechecked, area, confidence, quality, score) 
			values (:filename, :image, 0, 0, 0, 0, 0);
		`,
		sql.Named("filename", thumbName),
		sql.Named("image", thumbnail.image))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		insert or replace into
			thumbmap (filename, thumbname)
			values ( :filename, :thumbname );
		`,
		sql.Named("thumbname", thumbName),
		sql.Named("filename", filename))
	if err != nil {
		return err
	}

	return nil
}

// Adds thumbnail and metadata to database in a transaction
// You could consider updating more rows in a single transaction
// But how many rows at once? I do not know
// This has performed pretty reasonably in any case
// The limiting performance factor is elsewhere (handling media files)
func insertMedia(tx *sql.Tx, thumbnails []Thumbnail, metadata map[string]string) error {
	for _, thumbnail := range thumbnails {
		err := insertThumbnail(tx, metadata["diskfilename"], thumbnail)
		if err != nil {
			log.Println(err)
			return err
		}
	}

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

	for k, v := range metadata {
		_, err = stmtMetadata.Exec(metadata["diskfilename"], strings.ToLower(k), v)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

// Majority of this function is orchestrating the goroutines
// There may be opportunity to expand some error handling
// However have not seen enough errors in testing to work on
func AddFilesToDB(db *sql.DB, numWorkers int, probes int, path string) (int, error) {
	count := 0

	allFiles, err := recls(path)
	if err != nil {
		return count, err
	}

	filenames, err := filesNotInDB(db, allFiles)
	if err != nil {
		return count, err
	}

	if len(filenames) == 0 {
		return count, nil
	}

	type Reply struct {
		stopped bool
		err     error
		payload MediaInfo
	}

	type Request struct {
		stop     bool
		filename string
	}

	replies := make(chan Reply)
	requests := make(chan Request)

	worker := func() {
		for {
			select {
			case req := <-requests:
				if req.stop {
					replies <- Reply{stopped: true}
					return
				}

				mediainfo, err := ParseMediaFile(req.filename, probes)
				replies <- Reply{
					stopped: false,
					err:     err,
					payload: mediainfo,
				}
			}
		}
	}

	insertReply := func(db *sql.DB, reply Reply) error {
		_, err := db.Exec(`insert or replace into 
			filestat (filename, filesize) 
			values (:filename, :filesize);`,
			sql.Named("filename", reply.payload.filename),
			sql.Named("filesize", reply.payload.fileinfo.Size()))
		if err != nil {
			return err
		}

		if reply.err == errNotMediaFile {
			return nil
		}

		tx, err := db.Begin()
		if err != nil {
			log.Println(err)
			return err
		}
		defer tx.Rollback()

		err = insertMedia(tx, reply.payload.thumbnails, reply.payload.metadata)
		if err != nil {
			return err
		}

		// This is a kind of awkward way to do this
		_, err = tx.Exec(`insert or replace into 
			mediastat (
				filename, 
				canseek, 
				probes, 
				facechecked,
				bestthumb,
				bestscore
			) values (
				:filename, 
				:canseek, 
				:probes, 
				0,
				(
					select thumbname 
					from thumbmap 
					where filename = :filename
					limit 1
				),
				0
			);`,
			sql.Named("filename", reply.payload.filename),
			sql.Named("filesize", reply.payload.fileinfo.Size()),
			sql.Named("canseek", reply.payload.canseek),
			sql.Named("probes", probes))
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}

		err = Rescore(db, reply.payload.filename)
		if err != nil {
			log.Println(err)
			return err
		}

		return nil
	}

	workerCount := numWorkers

	replyHandler := func(reply Reply) error {
		if reply.stopped {
			workerCount--
			return nil
		}

		if errors.Is(reply.err, os.ErrNotExist) {
			return nil
		}

		if reply.err != nil && reply.err != errNotMediaFile {
			return reply.err
		}

		err = insertReply(db, reply)
		if err != nil {
			log.Println(err)
			return err
		}

		count++
		return nil
	}

	for i := 0; i < workerCount; i++ {
		go worker()
	}

	i := 0
	req := Request{
		stop:     false,
		filename: filenames[i],
	}

	for workerCount > 0 {
		select {
		case reply := <-replies:
			err := replyHandler(reply)
			if err != nil {
				return count, err
			}

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

	err = wordassocs(db)
	if err != nil {
		return count, err
	}

	err = fixtags(db)
	if err != nil {
		return count, err
	}

	err = cullMissing(db)
	if err != nil {
		return count, err
	}

	return count, err
}

func findMissingFiles(db *sql.DB) ([]string, error) {
	missing := make([]string, 0, 10)

	rows, err := db.Query(`select filename from filestat;`)
	if err != nil {
		log.Println(err)
		return missing, err
	}
	defer rows.Close()

	for rows.Next() {
		var filename string
		err = rows.Scan(&filename)
		if err != nil {
			return missing, err
		}

		// os.Stat returns PathErrors which don't always match os.ErrNotExist
		// trying os.Open instead
		fi, err := os.Open(filename)
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, filename)
			continue
		}

		if err != nil {
			return missing, err
		}

		err = fi.Close()
		if err != nil {
			return missing, err
		}
	}

	return missing, err
}

func cullMissing(db *sql.DB) error {
	missingFiles, err := findMissingFiles(db)
	if err != nil {
		log.Println(err)
		return err
	}

	if len(missingFiles) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

	delqueries := []string{
		"delete from tags where filename is ?;",
		"delete from wordassocs where filename is ?;",
		"delete from filestat where filename is ?;",
		"delete from mediastat where filename is ?;",
		"delete from thumbmap where filename is ?;",
	}

	count := 0
	for _, missingFile := range missingFiles {
		for _, delquery := range delqueries {
			_, err := tx.Exec(delquery, missingFile)
			if err != nil {
				log.Println(err)
				return err
			}
		}

		count += 1
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

// returns a slice of files which are not in the database
func filesNotInDB(db *sql.DB, filenames map[string]os.FileInfo) ([]string, error) {
	newFiles := make([]string, 0, len(filenames))

	rows, err := db.Query("select filename, filesize from filestat;")
	if err != nil {
		log.Println(err)
		return newFiles, err
	}
	defer rows.Close()

	dbfilesizes := make(map[string]int64)
	for rows.Next() {
		var filename string
		var filesize int64
		err = rows.Scan(&filename, &filesize)
		if err != nil {
			log.Println(err)
			return newFiles, err
		}

		dbfilesizes[filename] = filesize
	}

	for name, info := range filenames {
		dbsize, ok := dbfilesizes[name]
		if !ok || dbsize != info.Size() {
			newFiles = append(newFiles, name)
		}
	}

	return newFiles, nil
}

// changes all tag names to lowercase
// album, ALBUM, Album -> album
// artist, ARTIST, Artist -> artist
func fixtags(db *sql.DB) error {
	tags, err := util.AllRows1[string](db, "select distinct(name) from tags;")
	if err != nil {
		log.Println(err)
		return err
	}

	badTags := make([]string, 0, 0)
	for _, tag := range tags {
		lowered := strings.ToLower(tag)
		if lowered != tag {
			badTags = append(badTags, tag)
		}
	}

	if len(badTags) == 0 {
		return nil
	}

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

	for _, name := range badTags {
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
	filenames, contents, err := util.AllRows2[string, string](db, `
		select filename, val 
		from tags 
		where filename not in (
			select distinct(filename) 
			from wordassocs);`)
	if err != nil {
		log.Println(err)
		return err
	}

	if len(filenames) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	defer tx.Rollback()

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

	for i := 0; i < len(filenames); i++ {
		filename := filenames[i]
		words := contents[i]

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

func recls(dir string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo)

	badSuffixes := []string{"-wal", "-shm", "-journal"}

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

			ok := true
			for _, suffix := range badSuffixes {
				if strings.HasSuffix(entry.Name(), suffix) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}

			files[path], err = entry.Info()
			if err != nil {
				return err
			}
		}

		return nil
	}

	err := ls(dir)

	return files, err
}
