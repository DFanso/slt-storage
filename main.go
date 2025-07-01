package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

var conns = struct {
	sync.RWMutex
	m map[string]*websocket.Conn
}{m: make(map[string]*websocket.Conn)}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables")
	}

	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	// Create router
	r := gin.Default()

	// Load HTML templates
	r.LoadHTMLGlob("templates/*")

	// Serve static files
	r.Static("/static", "./static")

	// Routes
	r.GET("/", handleIndex)
	r.GET("/api/files", handleListFiles)
	r.POST("/api/upload", handleUpload)
	r.GET("/api/download", handleDownload)
	r.DELETE("/api/files", handleDeleteFile)
	r.POST("/api/folders", handleCreateFolder)
	r.GET("/ws/progress", handleWebSocket)

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting WebDAV Dashboard on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func handleUpload(c *gin.Context) {
	uploadID := c.PostForm("uploadID")
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing uploadID"})
		return
	}

	conns.RLock()
	ws, ok := conns.m[uploadID]
	conns.RUnlock()

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no websocket connection for this uploadID"})
		return
	}

	currentPath := c.PostForm("currentPath")
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chunkIndex := c.PostForm("chunkIndex")
	originalFilename := c.PostForm("originalFilename")
	totalSize, _ := strconv.ParseInt(c.PostForm("totalSize"), 10, 64)
	startOffset, _ := strconv.ParseInt(c.PostForm("startOffset"), 10, 64)

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	buf, err := io.ReadAll(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read chunk"})
		return
	}

	index, _ := strconv.Atoi(chunkIndex)
	err = uploadChunk(buf, index, originalFilename, ws, totalSize, startOffset, currentPath)
	if err != nil {
		log.Printf("!!! UPLOAD FAILED: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "chunk uploaded successfully"})
}

func handleListFiles(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/"
	}
	files, err := listFiles(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func handleDownload(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(path))
	c.Header("Content-Type", "application/octet-stream")

	err := downloadFile(path, c.Writer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func handleDeleteFile(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	err := deletePath(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
}

func handleCreateFolder(c *gin.Context) {
	path := c.PostForm("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	err := createDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "folder created successfully"})
}

func handleWebSocket(c *gin.Context) {
	uploadID := c.Query("id")
	if uploadID == "" {
		log.Println("ws upgrade error: missing upload id")
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("ws upgrade error:", err)
		return
	}
	defer ws.Close()

	conns.Lock()
	conns.m[uploadID] = ws
	conns.Unlock()

	defer func() {
		conns.Lock()
		delete(conns.m, uploadID)
		conns.Unlock()
		log.Println("WebSocket disconnected for:", uploadID)
	}()

	log.Printf("WebSocket connected for: %s", uploadID)

	// Keep the connection alive
	for {
		if _, _, err := ws.NextReader(); err != nil {
			ws.Close()
			break
		}
	}
}
