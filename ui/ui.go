package ui

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFiles embed.FS

// RegisterRoutes mounts the UI static files under the given prefix
func RegisterRoutes(r *gin.Engine, prefix string) {
	// Create a sub-filesystem rooted at "static"
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("failed to create static sub-filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	// Redirect bare prefix (no trailing slash) to prefix/
	r.GET(prefix, func(c *gin.Context) {
		c.Redirect(http.StatusFound, prefix+"/")
	})

	// Single catch-all handler covers both "/" and any asset path.
	// Do NOT use c.FileFromFS for index.html: http.FileServer redirects
	// any path ending in "/index.html" to "./" which creates a 301 loop.
	// Instead, read and write the bytes directly for the root case.
	r.GET(prefix+"/*filepath", func(c *gin.Context) {
		p := c.Param("filepath")
		if p == "/" || p == "" {
			content, err := staticFiles.ReadFile("static/index.html")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", content)
			return
		}
		c.Request.URL.Path = p
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
