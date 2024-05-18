package av

import (
	"testing"

	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

import _ "embed"

//go:embed testdata/subtitles.vtt
var subtitles string

func TestMissingFiles(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = InitDB(db)
	if err != nil {
		t.Fatal(err)
	}
}

// Test that text files and subtitle files are added
// Also testing those files being removed too!
func TestAddRemoveFiles(t *testing.T) {
	db, pathDir := createTestEnv(t)
	defer db.Close()
	defer os.RemoveAll(pathDir)

	pathText := createTextFile(t, pathDir)
	pathSub := createSubtitleFile(t, pathDir)

	n, err := AddFilesToDB(db, 1, 1, pathDir)
	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}

	for _, path := range []string{pathText, pathSub} {
		var pathCheck string
		err := db.QueryRow(`
			select filename 
			from filestat 
			where filename = :filepath;`,
			sql.Named("filepath", path)).Scan(&pathCheck)
		if err != nil {
			t.Fatal(err)
		}

		if pathCheck != path {
			t.Fatalf("expected %s, got %s", path, pathCheck)
		}

		err = db.QueryRow(`
			select filename 
			from mediastat 
			where filename = :filepath;`,
			sql.Named("filepath", path)).Scan(&pathCheck)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
	}

	err = os.Remove(pathText)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Remove(pathSub)
	if err != nil {
		t.Fatal(err)
	}

	err = cullMissing(db)
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{pathText, pathSub} {
		var pathCheck string
		err := db.QueryRow(`
			select filename 
			from filestat 
			where filename = :filepath;`,
			sql.Named("filepath", path)).Scan(&pathCheck)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
	}
}

func createTestEnv(t *testing.T) (*sql.DB, string) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	err = InitDB(db)
	if err != nil {
		t.Fatal(err)
	}

	pathDir, err := os.MkdirTemp(os.TempDir(), "http-server-av.test.")
	if err != nil {
		t.Fatal(err)
	}

	return db, pathDir
}

// ffmpeg treats text files as media, rendering them using a tty -> ansi demux/decode
// We would prefer to reject text files
func createTextFile(t *testing.T, pathDir string) string {
	tmpFile, err := os.CreateTemp(pathDir, "http-server-av.test.*.txt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tmpFile.WriteString("this is a text file")
	if err != nil {
		t.Fatal(err)
	}

	err = tmpFile.Close()
	if err != nil {
		t.Fatal(err)
	}

	return tmpFile.Name()
}

// Subtitles are media-adjacent
// ffmpeg will read them as media
// But for now we reject
func createSubtitleFile(t *testing.T, pathDir string) string {
	tmpFile, err := os.CreateTemp(pathDir, "http-server-av.test.*.vtt")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tmpFile.WriteString(subtitles)
	if err != nil {
		t.Fatal(err)
	}

	err = tmpFile.Close()
	if err != nil {
		t.Fatal(err)
	}

	return tmpFile.Name()
}
