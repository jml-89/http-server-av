//AddFilesToDB, the file
// All the functions that support AddFilesToDB and the orchestration of its goroutines
// Better than having it all encapsulated in a megafunction in another file

package av

import (
	"database/sql"
	"errors"
	"log"
	"os"
)

type request struct {
	stop     bool
	filename string
	probes   int
}

type reply struct {
	stopped bool
	err     error
	payload MediaInfo
	request request
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

	count, err = orchestrateParsers(db, numWorkers, probes, filenames)
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

func orchestrateParsers(db *sql.DB, numWorkers int, probes int, filenames []string) (int, error) {
	count := 0

	replies := make(chan reply)
	requests := make(chan request)
	workerCount := numWorkers

	for i := 0; i < workerCount; i++ {
		go parser(requests, replies)
	}

	i := 0
	req := request{
		stop:     false,
		filename: filenames[i],
		probes:   probes,
	}

	for workerCount > 0 {
		select {
		case reply := <-replies:
			if reply.stopped {
				workerCount--
				continue
			}

			err := insertReply(db, reply)
			if err != nil {
				log.Println(err)
				return count, err
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
	for {
		select {
		case req := <-requests:
			if req.stop {
				replies <- reply{stopped: true}
				return
			}

			mediainfo, err := ParseMediaFile(req.filename, req.probes)
			replies <- reply{
				stopped: false,
				err:     err,
				payload: mediainfo,
				request: req,
			}
		}
	}
}
