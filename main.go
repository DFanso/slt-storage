package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

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
	r.POST("/api/upload", handleUpload)
	r.GET("/api/files", handleListFiles)
	r.GET("/api/download/:filename", handleDownload)
	r.DELETE("/api/files/:filename", handleDeleteFile)

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
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chunkIndex := c.PostForm("chunkIndex")
	originalFilename := c.PostForm("originalFilename")

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	buf := make([]byte, file.Size)
	io.ReadFull(src, buf)

	index, _ := strconv.Atoi(chunkIndex)
	err = uploadChunk(buf, index, originalFilename)
	if err != nil {
		log.Printf("!!! UPLOAD FAILED: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "chunk uploaded successfully"})
}

func handleListFiles(c *gin.Context) {
	files, err := listFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func handleDownload(c *gin.Context) {
	filename := c.Param("filename")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")

	err := downloadFile(filename, c.Writer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func handleDeleteFile(c *gin.Context) {
	filename := c.Param("filename")
	err := deleteFile(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "file deleted"})
}
