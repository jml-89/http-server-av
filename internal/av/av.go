package av

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"encoding/hex"
	"golang.org/x/crypto/blake2b"

	"github.com/jml-89/http-server-av/internal/avc"
)

var errNotMediaFile = errors.New("File is not an image or video")
var errSeekFailed = errors.New("Seek failed")

// digest is the hex2str hash of the image
type Thumbnail struct {
	digest string
	source string
	image  []byte
}

type MediaInfo struct {
	filename  string
	fileinfo  os.FileInfo
	thumbnail Thumbnail
	metadata  map[string]string
	canseek   bool
}

func ParseMediaFile(filename string) (m MediaInfo, err error) {
	m.filename = filename

	m.fileinfo, err = os.Lstat(filename)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Println(err)
		}
		return
	}

	m.metadata, err = avc.GetMetadata(filename)
	if err != nil {
		if fmt.Sprintf("%s", err) == "Invalid data found when processing input" {
			// Just isn't a media file
			err = errNotMediaFile
			return
		}

		log.Printf("%s: %s", filename, err)
		return
	}

	if m.metadata["mediatype"] == "none" {
		err = errNotMediaFile
		return
	}

	m.thumbnail, m.canseek, err = CreateThumbnail(filename, 0.5)
	if err != nil {
		log.Printf("Failed to generate thumbnail for %s", filename)
		// We don't bail out here because it's not the end of the world
		// No thumbnail is no problem!
		err = nil
	}

	// We include these for the sake of the tags table
	m.metadata["favourite"] = "false"
	m.metadata["multithumbnail"] = "true"
	m.metadata["diskfiletime"] = m.fileinfo.ModTime().UTC().Format("2006-01-02T15:04:05")
	m.metadata["diskfilename"] = filename
	m.metadata["diskfilesize"] = fmt.Sprintf("%099d", m.fileinfo.Size())

	return
}

// Creates a number of evenly spaced out thumbnails from a video
func CreateThumbnails(pathIn string, num int) ([]Thumbnail, bool, error) {
	res := make([]Thumbnail, 0, num)

	step := 1.0 / float64(num)
	pos := step / 2.0
	for i := 0; i < num; i++ {
		thumb, seek, err := CreateThumbnail(pathIn, pos)
		if err != nil {
			log.Println(err)
			return nil, seek, err
		}

		res = append(res, thumb)

		if !seek {
			return res, seek, nil
		}

		pos += step
	}

	return res, true, nil
}

// Wrapper to handle various media situations
// Consider unseekable files, files with no video (audio files we'll call them), etc.
func CreateThumbnail(pathIn string, pos float64) (t Thumbnail, seek bool, err error) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "http-server-av.*.webp")
	if err != nil {
		log.Printf("%s", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	seek = true
	err = avc.CreateThumbnailX(pathIn, tmpFile.Name(), seek, pos)
	// Some streams don't support seeking
	// In this case just do a thumbnail of the first frame
	// Better than nothing
	if errors.Is(err, errSeekFailed) || fmt.Sprintf("%s", err) == "End of file" {
		seek = false
		err = avc.CreateThumbnailX(pathIn, tmpFile.Name(), seek, pos)
		if err != nil {
			log.Printf("%s: %s", pathIn, err)
		}
	}

	// When all else fails, go generic
	if err != nil {
		seek = false
		err = avc.CreateGenericThumbnail(tmpFile.Name())
		if err != nil {
			log.Printf("%s: %s", pathIn, err)
			return
		}
	}

	b, err := io.ReadAll(tmpFile)
	if err != nil {
		log.Printf("%s", err)
		return
	}

	digest, err := Checksum(b)
	if err != nil {
		log.Printf("%s", err)
		return
	}

	return Thumbnail{
		source: pathIn,
		digest: digest,
		image:  b,
	}, seek, err
}

func Checksum(b []byte) (string, error) {
	fi := bytes.NewReader(b)

	hasher, err := blake2b.New512(nil)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(hasher, fi)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func FileChecksum(path string) (string, error) {
	fi, err := os.Open(path)
	if err != nil {
		return "", err
	}

	hasher, err := blake2b.New512(nil)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(hasher, fi)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
