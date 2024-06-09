package controller

import (
	"comic-summaries/usecase"
	"github.com/labstack/echo/v4"
	"net/http"
)

type IComicController interface {
	GetComic(c echo.Context) error
	GetAllComics(c echo.Context) error
	SearchComics(c echo.Context) error
}

type comicController struct {
	cu usecase.IComicUsecase
}

func NewComicController(cu usecase.IComicUsecase) IComicController {
	return &comicController{cu}
}

func (cc *comicController) GetComic(c echo.Context) error {
	id := c.Param("id")
	comic, err := cc.cu.GetComicByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, comic)
}

func (cc *comicController) GetAllComics(c echo.Context) error {
	comics, err := cc.cu.GetAllComics(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, comics)
}

func (cc *comicController) SearchComics(c echo.Context) error {
	title := c.QueryParam("title")
	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Title query parameter is required"})
	}
	comics, err := cc.cu.SearchComicsByTitle(c.Request().Context(), title)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, comics)
}
