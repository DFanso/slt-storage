package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/studio-b12/gowebdav"
)

type WebDAVFile struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

type ProgressMessage struct {
	TotalWritten int64 `json:"totalWritten"`
	TotalSize    int64 `json:"totalSize"`
}

func newWebdavClient() *gowebdav.Client {
	webdavURL := os.Getenv("WEBDAV_URL")
	webdavUser := os.Getenv("WEBDAV_USER")
	webdavPassword := os.Getenv("WEBDAV_PASSWORD")
	return gowebdav.NewClient(webdavURL, webdavUser, webdavPassword)
}

func listFiles(path string) ([]WebDAVFile, error) {
	c := newWebdavClient()
	files, err := c.ReadDir(path)
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]WebDAVFile)
	for _, f := range files {
		fullPath := filepath.ToSlash(filepath.Join(path, f.Name()))
		if f.IsDir() {
			fileMap[fullPath] = WebDAVFile{Name: f.Name(), Path: fullPath, IsDir: true}
			continue
		}

		if strings.Contains(f.Name(), ".chunk") {
			originalName := strings.Split(f.Name(), ".chunk")[0]
			originalPath := filepath.ToSlash(filepath.Join(path, originalName))
			if existing, exists := fileMap[originalPath]; !exists {
				fileMap[originalPath] = WebDAVFile{Name: originalName, Path: originalPath, IsDir: false, Size: f.Size()}
			} else {
				existing.Size += f.Size()
				fileMap[originalPath] = existing
			}
		} else {
			fileMap[fullPath] = WebDAVFile{Name: f.Name(), Path: fullPath, IsDir: false, Size: f.Size()}
		}
	}

	var result []WebDAVFile
	for _, file := range fileMap {
		result = append(result, file)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func createDir(path string) error {
	c := newWebdavClient()
	return c.MkdirAll(path, 0755)
}

func uploadChunk(chunk []byte, chunkIndex int, originalFilename string, ws *websocket.Conn, totalSize int64, startOffset int64, currentPath string) error {
	c := newWebdavClient()
	chunkName := fmt.Sprintf("%s.chunk%d", originalFilename, chunkIndex)
	path := filepath.ToSlash(filepath.Join(currentPath, chunkName))

	err := c.Write(path, chunk, 0644)
	if err != nil {
		return err
	}

	progress := ProgressMessage{
		TotalWritten: startOffset + int64(len(chunk)),
		TotalSize:    totalSize,
	}

	msg, _ := json.Marshal(progress)
	ws.WriteMessage(websocket.TextMessage, msg)

	return nil
}

func downloadFile(path string, w io.Writer) error {
	c := newWebdavClient()
	dir := filepath.ToSlash(filepath.Dir(path))
	base := filepath.Base(path)

	files, err := c.ReadDir(dir)
	if err != nil {
		if dir == "." {
			files, err = c.ReadDir("/")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	var chunks []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), base+".chunk") {
			chunks = append(chunks, filepath.ToSlash(filepath.Join(dir, f.Name())))
		}
	}

	if len(chunks) == 0 {
		rc, err := c.ReadStream(path)
		if err != nil {
			return err
		}
		defer rc.Close()
		_, err = io.Copy(w, rc)
		return err
	}

	sort.Slice(chunks, func(i, j int) bool {
		a := strings.Split(chunks[i], ".chunk")
		b := strings.Split(chunks[j], ".chunk")
		numA, _ := strconv.Atoi(a[len(a)-1])
		numB, _ := strconv.Atoi(b[len(b)-1])
		return numA < numB
	})

	for _, chunkPath := range chunks {
		rc, err := c.ReadStream(chunkPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func deletePath(path string) error {
	c := newWebdavClient()
	stat, err := c.Stat(path)
	if err == nil && stat.IsDir() {
		return c.RemoveAll(path)
	}

	dir := filepath.ToSlash(filepath.Dir(path))
	base := filepath.Base(path)

	files, err := c.ReadDir(dir)
	if err != nil {
		if dir == "." {
			files, err = c.ReadDir("/")
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	deletedSomething := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), base) {
			chunkPath := filepath.ToSlash(filepath.Join(dir, f.Name()))
			if err := c.Remove(chunkPath); err == nil {
				deletedSomething = true
			}
		}
	}

	if !deletedSomething {
		return c.Remove(path)
	}

	return nil
}
