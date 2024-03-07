package main

import (
	"comic-summaries/controller"
	"comic-summaries/handler"
	"comic-summaries/repository"
	"comic-summaries/usecase"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Echoインスタンスの作成
	e := echo.New()

	// ミドルウェアの設定
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// リポジトリのインスタンス化
	// DynamoDBを使用する場合は、ここでDynamoDBクライアントを初期化してリポジトリに渡す
	comicRepo := repository.NewComicRepository()

	// ユースケースのインスタンス化
	comicUsecase := usecase.NewComicUsecase(comicRepo)

	comicController := controller.NewComicController(comicUsecase)

	// ハンドラの登録
	handler.NewComicHandler(e, comicController)

	// サーバーの起動
	e.Logger.Fatal(e.Start(":1323"))
}
