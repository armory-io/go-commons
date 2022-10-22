package server

import (
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func spaMiddleware(spaConfig SPAConfiguration) gin.HandlerFunc {
	index := "/"

	fs := static.LocalFile(spaConfig.Directory, false)
	fileServer := http.FileServer(fs)

	var prefix string
	if spaConfig.Prefix != "" {
		prefix = strings.TrimSpace(spaConfig.Prefix)
		prefix = strings.TrimSuffix(prefix, "/")
		prefix = strings.TrimPrefix(prefix, "/")
		prefix = "/" + prefix
	}

	if prefix != "/" {
		fileServer = http.StripPrefix(spaConfig.Prefix, fileServer)
		index = prefix + index
	}
	return func(c *gin.Context) {
		if fs.Exists(spaConfig.Prefix, c.Request.URL.Path) {
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		} else {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				c.Request.URL.Path = index
				fileServer.ServeHTTP(c.Writer, c.Request)
				c.Abort()
			}
		}
	}
}
