package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var processingFilesList = struct {
	sync.Mutex
	files map[string]struct{}
}{
	files: make(map[string]struct{}),
}

func addProcessingFile(fileName string) {
	processingFilesList.Lock()
	defer processingFilesList.Unlock()
	processingFilesList.files[fileName] = struct{}{}
}

func removeProcessingFile(fileName string) {
	processingFilesList.Lock()
	defer processingFilesList.Unlock()
	delete(processingFilesList.files, fileName)
}

func isProcessing(fileName string) bool {
	processingFilesList.Lock()
	defer processingFilesList.Unlock()
	_, exists := processingFilesList.files[fileName]
	return exists
}

func AllowListMiddleware() gin.HandlerFunc {
	var allowList = []string{"127.0.0.1"}
	return func(c *gin.Context) {
		clientIp := c.ClientIP()
		allow := slices.Contains(allowList, clientIp)
		if !allow {
			c.String(http.StatusForbidden, "Access Forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}

func ArchiveByYTDLP() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlParam := c.Query("url")

		if urlParam == "" {
			c.String(http.StatusBadRequest, "Missing 'url' query param")
			return
		}

		// Get the filename without downloading the video
		cmd := exec.Command("yt-dlp", "--get-filename", "-o", "%(title)s[%(id)s].%(ext)s", urlParam)
		output, err := cmd.Output()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error getting filename")
			return
		}

		fileName := strings.TrimSuffix(string(output), "\n")

		// Check if file is in process of archive
		if isProcessing(fileName) {
			c.String(http.StatusBadRequest, "File '%s' is already being processed", fileName)
			return
		}

		// Check if the file exists
		filePath := filepath.Join("downloads", fileName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			go func(name string, outputPath string, requestUrl string) {
				addProcessingFile(name)

				// File does not exist, download it
				downloadCmd := exec.Command("yt-dlp", "--cookies", "cookies.txt", "--format", "best", "-o", outputPath, requestUrl)
				downloadErr := downloadCmd.Run()
				if downloadErr != nil {
					fmt.Printf("Error archiving %s: %v\n", filePath, downloadErr)
				}
				removeProcessingFile(name)
			}(fileName, filePath, urlParam)
			c.String(http.StatusOK, "File '%s' is being processed", fileName)
			return
		}

		c.File(filePath)

	}
}

func Archive() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlParam := c.Query("url")
		if urlParam == "" {
			c.String(http.StatusBadRequest, "Missing 'url' query param")
			return
		}

		fileName := filepath.Base(urlParam)

		// Check if file is in process of archive
		if isProcessing(fileName) {
			c.String(http.StatusBadRequest, "File '%s' is already being processed", fileName)
			return
		}

		filePath := filepath.Join("downloads", fileName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// File not found locally, download it
			go func(name string, outputPath string, requestUrl string) {
				addProcessingFile(name)
				cmd := exec.Command("curl",
					"-b", "cookies.txt",
					"-o", outputPath,
					"--url", requestUrl,
					"-H", "User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
					"-H", "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8,video/*;q=0.8",
					"-H", "Accept-Language: en-US,en;q=0.5",
					"-H", "Accept-Encoding: gzip, deflate, br",
					"-H", "DNT: 1",
					"-H", "Connection: keep-alive",
					"-H", "Upgrade-Insecure-Requests: 1",
					"-H", "Sec-Fetch-Dest: document",
					"-H", "Sec-Fetch-Mode: navigate",
					"-H", "Sec-Fetch-Site: none",
					"-H", "Sec-Fetch-User: ?1",
					"-H", "Sec-GPC: 1",
					"-H", "Pragma: no-cache",
					"-H", "Cache-Control: no-cache",
					"-H", "TE: trailers",
				)
				err := cmd.Run()
				if err != nil {
					fmt.Printf("Error archiving %s: %v\n", filePath, err)
				}
				removeProcessingFile(name)
			}(fileName, filePath, urlParam)
			c.String(http.StatusOK, "File '%s' is being processed", fileName)
			return
		}

		c.File(filePath)

	}
}

func main() {
	_, err := os.Stat("cookies.txt")

	if err != nil {
		fmt.Println("cookies.txt file missing")
		return
	}

	r := gin.Default()

	r.Use(AllowListMiddleware())

	r.GET("/yt-dlp", ArchiveByYTDLP())
	r.GET("/", Archive())

	r.StaticFS("/fs", gin.Dir("downloads", true))

	r.Run()
}
