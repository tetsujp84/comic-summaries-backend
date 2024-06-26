package handler

import (
	"comic-summaries/controller"
	"github.com/labstack/echo/v4"
)

func NewComicHandler(e *echo.Echo, cc controller.IComicController) {
	e.GET("/summaries/:id", cc.GetComic)
	e.GET("/summaries", cc.GetAllComics)
	e.GET("/search", cc.SearchComics)
	e.GET("/count", cc.GetTotalCount)

	e.GET("/echo", cc.Echo)
}
