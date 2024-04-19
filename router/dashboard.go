package router

import (
	"one-api/controller"
	"one-api/middleware"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetDashboardRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	apiRouter := router.Group("/")
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	apiRouter.Use(middleware.OpenaiAuth())
	{
		apiRouter.GET("/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/v1/dashboard/billing/subscription", controller.GetSubscription)
		apiRouter.GET("/dashboard/billing/usage", controller.GetUsage)
		apiRouter.GET("/v1/dashboard/billing/usage", controller.GetUsage)
	}
}
