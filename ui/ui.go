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

	// Serve index.html at the prefix root
	r.GET(prefix, func(c *gin.Context) {
		c.Redirect(http.StatusFound, prefix+"/")
	})
	r.GET(prefix+"/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(subFS))
	})

	// Serve all other static assets
	r.GET(prefix+"/*filepath", func(c *gin.Context) {
		filepath := c.Param("filepath")
		c.Request.URL.Path = filepath
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
