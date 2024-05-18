//Evaluator
// Goes through thumbnails, finds faces, saves face discovery information to the database

//Improver
// Finds media files which have poor thumbnails and adds more thumbnails
// (hoping the new ones will be better)

package av

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"

	"github.com/jml-89/http-server-av/internal/avc"
	"github.com/jml-89/http-server-av/internal/util"
)

func RescoreAll(db *sql.DB) error {
	_, err := db.Exec(`update thumbnail set 
		score = min(50000, area) 
			* min(0.3, quality) 
			* confidence;`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`update mediastat set bestscore = 0 where bestscore is null;`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`update mediastat set 
		bestthumb = case when a.score > bestscore then a.thumbname else bestthumb end,
		bestscore = max(bestscore, a.score)
		from (
			select x.filename, x.thumbname, y.score
			from thumbmap x
			inner join thumbnail y
			on x.thumbname = y.thumbname
		) a
		where mediastat.bestthumb is null
		and a.filename = mediastat.filename;`)
	if err != nil {
		return err
	}

	return nil
}

func Rescore(db *sql.DB, filename string, skipThumbScore bool) error {
	stmts := []string{
		`update thumbnail set 
			score = min(50000, area) * min(0.3, quality) * confidence
		where thumbname in (
			select thumbname 
			from thumbmap 
			where filename = :filename);`,

		`update mediastat set 
			bestthumb = b.thumbname,
			bestscore = b.score
		from (
			select a.thumbname, b.score
			from thumbmap a
			inner join thumbnail b
			on a.filename = :filename
			and a.thumbname = b.thumbname
			order by b.score desc
			limit 1
		) b
		where filename = :filename;`,
	}

	if skipThumbScore {
		stmts = stmts[1:]
	}

	for _, stmt := range stmts {
		_, err := db.Exec(stmt, sql.Named("filename", filename))
		if err != nil {
			return err
		}
	}

	return nil
}

type Evaluator struct {
	tmb *avc.Thumbnailer
}

func NewEvaluator(threadCount int) (Evaluator, error) {
	tmb, err := avc.NewThumbnailer(threadCount)
	return Evaluator{tmb: tmb}, err
}

func (e *Evaluator) Run(db *sql.DB) (int, error) {
	count := 0

	filenames, err := util.AllRows1(db,
		`select filename from mediastat where not facechecked;`,
		"",
	)

	for _, filename := range filenames {
		err = e.evaluate(db, filename)
		if err != nil {
			return count, err
		}

		err = Rescore(db, filename, false)
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
		`select filename, probes
		from mediastat
		where facechecked
		and canseek
		and probes < 16
		and bestscore < 15000.0;`, "", int(0))
	if err != nil {
		log.Println(err)
		return count, err
	}

	for i := 0; i < len(filenames); i++ {
		filename := filenames[i]
		probes := probeCounts[i]

		probes = probes * 2

		thumbnails, canseek, err := CreateThumbnails(filename, probes)
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
			update mediastat
			set 
				probes = :probes,
				canseek = :canseek,
				facechecked = 0
			where 
				filename = :filename;
			`,
			sql.Named("filename", filename),
			sql.Named("probes", probes),
			sql.Named("canseek", canseek))
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
		select thumbname, image
		from thumbnail
		where not facechecked
		and thumbname in (
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

		_, err = tx.Exec(`update thumbnail set
					facechecked = 1,
					area = sum(b.area),
					confidence = avg(b.confidence),
					quality = avg(b.quality)
				from ( 
					select area, confidence, quality
					from thumbface
					where thumbname = :thumbname
				) as b
				where thumbname = :thumbname;`,
			sql.Named("thumbname", thumbname))
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	_, err = db.Exec(`update mediastat set
			facechecked = 1
			where filename = :filename;`,
		sql.Named("filename", filename))
	if err != nil {
		return err
	}

	return nil
}
