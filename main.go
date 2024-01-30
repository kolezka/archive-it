package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

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

		// Check if the file exists
		filePath := filepath.Join("downloads", fileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// File does not exist, download it
			downloadCmd := exec.Command("yt-dlp", "--format", "best", "-o", filePath, urlParam)
			downloadErr := downloadCmd.Run()
			if downloadErr != nil {
				c.String(http.StatusInternalServerError, "Error downloading video")
				return
			}
		}

		c.String(http.StatusOK, "Archived successfully: %s", fileName)
	}
}

func Archive() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlParam := c.Query("url")
		if urlParam == "" {
			c.String(http.StatusBadRequest, "Missing 'url' query param")
			return
		}

		ext := filepath.Ext(urlParam)
		if ext == "" {
			c.String(http.StatusBadRequest, "Unable to determine file extension")
			return
		}

		fileName := filepath.Base(urlParam)
		filePath := filepath.Join("downloads", fileName)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// File not found locally, download it
			cmd := exec.Command("wget", "-O", filePath, urlParam)
			err := cmd.Run()
			if err != nil {
				c.String(http.StatusInternalServerError, "Error downloading media")
				return
			}
		}

		c.String(http.StatusOK, "Archived successfully %s", fileName)

	}
}

func main() {
	r := gin.Default()

	r.Use(AllowListMiddleware())

	r.GET("/yt-dlp", ArchiveByYTDLP())
	r.GET("/", Archive())

	r.StaticFS("/fs", gin.Dir("downloads", true))

	r.Run()
}
