package controller

import (
	"comic-summaries/usecase"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

type IComicController interface {
	GetComic(c echo.Context) error
	GetAllComics(c echo.Context) error
	SearchComics(c echo.Context) error
	GetTotalCount(c echo.Context) error
	Echo(c echo.Context) error
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
	pageParam := c.QueryParam("page")
	page, err := strconv.Atoi(pageParam)
	if err != nil || page < 1 {
		page = 1
	}
	comics, lastEvaluatedKey, err := cc.cu.GetAllComics(c.Request().Context(), page)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"comics":           comics,
		"lastEvaluatedKey": lastEvaluatedKey,
	})
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

func (cc *comicController) GetTotalCount(c echo.Context) error {
	totalCount, err := cc.cu.GetTotalCount(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]int{"count": totalCount})
}

func (cc *comicController) Echo(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
