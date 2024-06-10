package main

import (
	"comic-summaries/controller"
	"comic-summaries/handler"
	"comic-summaries/repository"
	"comic-summaries/usecase"
	"github.com/joho/godotenv"
	"log"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Echoインスタンスの作成
	e := echo.New()

	// デバッグのため強制終了
	if os.Getenv("GO_ENV") == "dev" {
		err := godotenv.Load()
		if err != nil {
			log.Fatalln(err)
		}
	}

	// ミドルウェアの設定
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	frontendEndPoint := os.Getenv("FRONTEND_ENDPOINT")
	log.Printf("Frontend Endpoint: %s", frontendEndPoint) // デバッグ出力
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{frontendEndPoint}, // Reactアプリのオリジン
		AllowMethods: []string{echo.GET, echo.PUT, echo.POST, echo.DELETE},
	}))

	// 静的ファイルの設定
	e.Static("/images", "images")

	// リポジトリのインスタンス化
	// DynamoDBを使用する場合は、ここでDynamoDBクライアントを初期化してリポジトリに渡す
	comicRepo := repository.NewComicRepository()

	// ユースケースのインスタンス化
	comicUsecase := usecase.NewComicUsecase(comicRepo)

	comicController := controller.NewComicController(comicUsecase)

	// ハンドラの登録
	handler.NewComicHandler(e, comicController)

	// サーバーの起動
	port := os.Getenv("PORT")
	if port == "" {
		port = "1323" // デフォルトポート
	}
	e.Logger.Fatal(e.Start(":" + port))
}
