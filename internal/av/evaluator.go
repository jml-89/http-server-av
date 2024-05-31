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
	"math"
	"math/rand"
	"path/filepath"

	"github.com/jml-89/http-server-av/internal/avc"
	"github.com/jml-89/http-server-av/internal/util"
)

func ScoreFunc(area int, confidence float64, quality float64) float64 {
	return math.Sqrt(math.Max(0.0, float64(area))) * confidence * quality
}

func ThumbCull(db *sql.DB, filename string) error {
	thumbnames, err := util.AllRows1[string](db, `
		with thebest as (
			select 
				thumbmap.thumbname
			from 
				thumbmap 
			inner join
				thumbnail
			on 
				thumbmap.filename = :filename
			and
				thumbmap.thumbname = thumbnail.thumbname
			order by 
				thumbnail.score desc
			limit
				4
		), therest as (
			select
				thumbmap.thumbname
			from 
				thumbmap 
			where	
				thumbmap.filename = :filename
			and
				thumbmap.thumbname not in (
					select 
						thumbname 
					from 
						thebest
				)
		) select thumbname from therest;`, sql.Named("filename", filename))
	if err != nil {
		return err
	}

	if len(thumbnames) == 0 {
		return nil
	}

	stmts := []string{
		`delete from thumbface where thumbname = :thumbname`,
		`delete from thumbnail where thumbname = :thumbname`,
		`delete from thumbmap where thumbname = :thumbname`,
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, thumbname := range thumbnames {
		for _, stmt := range stmts {
			_, err = tx.Exec(stmt, sql.Named("thumbname", thumbname))
			if err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func RescoreAll(db *sql.DB) error {
	_, err := db.Exec(`update thumbnail set 
		score = scorefn(area, confidence, quality);`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`update mediastat set 
		bestthumb = a.thumbname,
		bestscore = max(a.score)
		from (
			select x.filename, x.thumbname, y.score
			from thumbmap x
			inner join thumbnail y
			on x.thumbname = y.thumbname
		) a
		where a.filename = mediastat.filename;`)
	if err != nil {
		return err
	}

	return nil
}

func Rescore(db *sql.DB, filename string) error {
	stmts := []string{
		`update thumbnail set 
			score = scorefn(area, confidence, quality)
		where thumbname in (
			select thumbname 
			from thumbmap 
			where filename = :filename);`,

		`update mediastat set 
			bestthumb = a.thumbname,
			bestscore = max(a.score)
		from (
			select x.filename, x.thumbname, y.score
			from thumbmap x
			inner join thumbnail y
			on x.filename = :filename
			and x.thumbname = y.thumbname
		) a
		where a.filename = mediastat.filename;`,
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

	filenames, err := util.AllRows1[string](db,
		`select filename from mediastat where not facechecked;`,
	)

	for _, filename := range filenames {
		err = e.evaluate(db, filename)
		if err != nil {
			return count, err
		}

		err = Rescore(db, filename)
		if err != nil {
			return count, err
		}

		err = ThumbCull(db, filename)
		if err != nil {
			return count, err
		}

		count += 1
	}

	return count, nil
}

func Improver(db *sql.DB) (int, error) {
	count := 0

	filenames, probeCounts, err := util.AllRows2[string, int](db,
		`select filename, probes
		from mediastat
		where facechecked
		and canseek
		and ((probes < 10) or (probes < 30 and bestscore > 0))
		and bestscore < scorefn(30000, 0.85, 0.6)
		order by probes asc;`)
	if err != nil {
		log.Println(err)
		return count, err
	}

	for i, _ := range filenames {
		filename := filenames[i]
		probes := probeCounts[i]

		probes += 1

		thumbnail, canseek, err := CreateThumbnail(filename, rand.Float64())
		if err != nil {
			log.Printf("Failed to generate thumbnail for %s", filename)
			return count, err
		}

		tx, err := db.Begin()
		if err != nil {
			return count, err
		}
		defer tx.Rollback()

		//for _, thumbnail := range thumbnails {
		err = insertThumbnail(tx, filename, thumbnail)
		if err != nil {
			return count, err
		}
		//}

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
	thumbnames, err := util.AllRows1[string](db, `
		select thumbname
		from thumbnail
		where not facechecked
		and thumbname in (
			select thumbname
			from thumbmap
			where filename = :filename);
		`, sql.Named("filename", filename))
	if err != nil {
		return err
	}

	for _, thumbname := range thumbnames {
		thumbpath := filepath.Join(".thumbs", thumbname)

		faces, err := e.tmb.RunImage(thumbpath)
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

		stmt := `update thumbnail set
				facechecked = 1,
				area = sum(b.area),
				confidence = avg(b.confidence),
				quality = avg(b.quality)
			from ( 
				select area, confidence, quality
				from thumbface
				where thumbname = :thumbname
			) as b
			where thumbname = :thumbname;`
		if len(faces) == 0 {
			stmt = `update thumbnail set facechecked = 1 where thumbname = :thumbname;`
		}

		_, err = tx.Exec(stmt, sql.Named("thumbname", thumbname))
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
