//Evaluator 
// Goes through thumbnails, finds faces, saves face discovery information to the database

//Improver
// Finds media files which have poor thumbnails and adds more thumbnails
// (hoping the new ones will be better)

package av

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	//"os"
	"log"

	"github.com/jml-89/http-server-av/internal/util"
)

type Evaluator struct {
	tmb *Thumbnailer
}

func NewEvaluator(threadCount int) (Evaluator, error) {
	tmb, err := NewThumbnailer(threadCount)
	return Evaluator { tmb: tmb }, err
}

func (e *Evaluator) Run(db *sql.DB) (int, error) {
	count := 0

	filenames, err := util.AllRows1(db,
		`select filename from filestats where not facechecked;`, 
		"",
	)

	for _, filename := range filenames {
		err = e.evaluate(db, filename)
		if err != nil {
			return count, err
		}

		count += 1
	}

	return count, nil
}

func Improver(db *sql.DB) (int, error) {
	count := 0

	filenames, probeCounts, err := util.AllRows2(db,
		`select a.filename, a.probes
		from filestats a
		where a.facechecked
		and a.probes < 16
		and a.filename not in (
			select b.filename
			from bestthumb b
			where b.filename = a.filename
			and b.score > 15000.0
		);`, "", int(0))
	if err != nil {
		log.Println(err)
		return count, err
	}

	for i := 0; i < len(filenames); i++ {
		filename := filenames[i]
		probes := probeCounts[i]

		probes = probes * 2

		thumbnails, err := CreateThumbnails(filename, probes)
		if err != nil {
			log.Printf("Failed to generate thumbnail for %s", filename)
			return count, err
		}

		tx, err := db.Begin()
		if err != nil {
			return count, err
		}
		defer tx.Rollback()

		for _, thumbnail := range thumbnails {
			err = insertThumbnail(tx, filename, thumbnail)
			if err != nil {
				return count, err
			}
		}

		_, err = tx.Exec(`
			update filestats
			set 
				probes = :probes,
				facechecked = 0
			where 
				filename = :filename;
			`,
			sql.Named("filename", filename),
			sql.Named("probes", probes))
		if err != nil {
			return count, err
		}

		err = tx.Commit()
		if err != nil {
			return count, err
		}

		count += 1
	}

	return count, nil
}

func (e *Evaluator) evaluate(db *sql.DB, filename string) error {
	thumbnames, images, err := util.AllRows2(db, `
		select filename, image
		from thumbnails
		where not facechecked
		and filename in (
			select thumbname
			from thumbmap
			where filename = :filename);
		`, "", []byte(""), sql.Named("filename", filename))
	if err != nil {
		return err
	}

	for i := 0; i < len(thumbnames); i++ {
		thumbname := thumbnames[i]
		image := images[i]

		faces, err := e.tmb.RunImageBuf(image)
		if err != nil {
			log.Println(err)
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		for _, face := range faces {
			_, err = tx.Exec(`
				insert or replace into
				thumbface ( thumbname, area, confidence, quality )
				values ( :thumbname, :area, :confidence, :quality );
			`, 
			sql.Named("thumbname", thumbname),
			sql.Named("area", face.Area),
			sql.Named("confidence", face.Confidence),
			sql.Named("quality", face.Quality))
		}

		_, err = tx.Exec(`update thumbnails set
				facechecked = 1
				where filename = :thumbname;`,
			sql.Named("thumbname", thumbname))
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	_, err = db.Exec(`update filestats set
			facechecked = 1
			where filename = :filename;`,
		sql.Named("filename", filename))
	if err != nil {
		return err
	}

	return nil
}

