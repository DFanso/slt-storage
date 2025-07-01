package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/studio-b12/gowebdav"
)

func newWebdavClient() *gowebdav.Client {
	webdavURL := os.Getenv("WEBDAV_URL")
	webdavUser := os.Getenv("WEBDAV_USER")
	webdavPassword := os.Getenv("WEBDAV_PASSWORD")
	return gowebdav.NewClient(webdavURL, webdavUser, webdavPassword)
}

func listFiles() ([]string, error) {
	c := newWebdavClient()
	files, err := c.ReadDir("/")
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]bool)
	for _, f := range files {
		if strings.Contains(f.Name(), ".chunk") {
			originalName := strings.Split(f.Name(), ".chunk")[0]
			fileMap[originalName] = true
		} else {
			fileMap[f.Name()] = true
		}
	}

	var result []string
	for name := range fileMap {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func uploadChunk(chunk []byte, chunkIndex int, originalFilename string) error {
	c := newWebdavClient()
	path := fmt.Sprintf("/%s.chunk%d", originalFilename, chunkIndex)
	return c.Write(path, chunk, 0644)
}

func downloadFile(filename string, w io.Writer) error {
	c := newWebdavClient()
	files, err := c.ReadDir("/")
	if err != nil {
		return err
	}

	var chunks []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), filename+".chunk") {
			chunks = append(chunks, f.Name())
		}
	}

	// Sort chunks by index
	sort.Slice(chunks, func(i, j int) bool {
		a := strings.Split(chunks[i], ".chunk")
		b := strings.Split(chunks[j], ".chunk")
		numA, _ := strconv.Atoi(a[1])
		numB, _ := strconv.Atoi(b[1])
		return numA < numB
	})

	for _, chunkName := range chunks {
		rc, err := c.ReadStream(fmt.Sprintf("/%s", chunkName))
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteFile(filename string) error {
	c := newWebdavClient()
	files, err := c.ReadDir("/")
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.HasPrefix(f.Name(), filename) {
			err := c.Remove(fmt.Sprintf("/%s", f.Name()))
			if err != nil {
				// Log error but continue trying to delete other chunks
				fmt.Printf("failed to delete chunk %s: %v\n", f.Name(), err)
			}
		}
	}

	return nil
}
