package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	api := r.Group("/api")
	{
		api.POST("/project", h.SetProject)
		api.GET("/filetree", h.GetFileTree)
		api.GET("/pods", h.GetPods)
		api.GET("/pod/*path", h.GetPod)
		api.GET("/containers/*path", h.GetContainers)
		api.GET("/container/*path", h.GetContainer)
		api.GET("/dependencies/*path", h.GetDependencies)
	}

	return r
}
