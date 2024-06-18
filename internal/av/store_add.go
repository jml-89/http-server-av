//AddFilesToDB, the file
// All the functions that support AddFilesToDB and the orchestration of its goroutines
// Better than having it all encapsulated in a megafunction in another file

package av

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"sync"
	"path/filepath"
	"strings"
)

type request struct {
	filename string
	probes   int
}

type reply struct {
	err     error
	payload MediaInfo
	request request
}

// Majority of this function is orchestrating the goroutines
// There may be opportunity to expand some error handling
// However have not seen enough errors in testing to work on
func AddFilesToDB(db *sql.DB, ignore []string, numWorkers int, path string) (int, error) {
	count := 0

	allFiles, err := recls(path, ignore)
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

	count, err = orchestrateParsers(db, numWorkers, filenames)
	if err != nil {
		return count, err
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

func orchestrateParsers(db *sql.DB, numWorkers int, filenames []string) (int, error) {
	count := 0

	replies := make(chan reply)
	requests := make(chan request)
	workerCount := numWorkers

	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			parser(requests, replies)
		}()
	}

	go func() {
		for _, filename := range filenames {
			requests<- request{ filename: filename, probes: 1 }
		}
		close(requests)

		wg.Wait()
		close(replies)
	}()

	for reply := range replies {
		err := insertReply(db, reply)
		if err != nil {
			log.Println(err)
			return count, err
		}

		count++
	}

	return count, nil
}

func insertReply(db *sql.DB, reply reply) error {
	if errors.Is(reply.err, os.ErrNotExist) {
		return nil
	}

	if reply.err != nil && reply.err != errNotMediaFile {
		return reply.err
	}

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

	err = insertMedia(tx, []Thumbnail{reply.payload.thumbnail}, reply.payload.metadata)
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
		sql.Named("probes", reply.request.probes))
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func parser(requests <-chan request, replies chan<- reply) {
	for req := range requests {
		mediainfo, err := ParseMediaFile(req.filename)
		replies <- reply{
			err:     err,
			payload: mediainfo,
			request: req,
		}
	}
}

func recls(dir string, ignores []string) (map[string]os.FileInfo, error) {
	files := make(map[string]os.FileInfo)

	badSuffixes := []string{"-wal", "-shm", "-journal"}

	var ls func(string) error
	ls = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			ok := true
			for _, ignore := range ignores {
				if entry.Name() == ignore {
					ok = false
					break
				}
			}

			if !ok {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				err = ls(path)
				if err != nil {
					return err
				}
				continue
			}

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
