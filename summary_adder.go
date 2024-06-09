package main

import (
	"comic-summaries/entity"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	popularTitles, imagePaths := scrapeComicTitlesAndImages("https://comic.k-manga.jp/search/magazine/43", 5)
	mangaData := getComicSummaries(popularTitles, imagePaths, os.Getenv("OPENAI_API_KEY"))
	storeComicData(mangaData)
}

func scrapeComicTitlesAndImages(url string, count int) ([]string, []string) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error fetching URL: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Fatalf("Error: Status code %d", res.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatalf("Error loading HTTP response body: %v", err)
	}

	var titles []string
	var imagePaths []string
	doc.Find(".book-list--item").Each(func(i int, s *goquery.Selection) {
		title := s.Find(".book-list--title").Text()
		imageUrl, exists := s.Find(".book-list--img").Attr("src")
		if exists {
			imagePath, err := downloadImage(imageUrl)
			if err != nil {
				log.Printf("Error downloading image: %v", err)
			} else {
				titles = append(titles, title)
				imagePaths = append(imagePaths, imagePath)
				fmt.Println(title, imagePath)
			}
		}
	})

	popularTitles := titles[:count]
	popularImagePaths := imagePaths[:count]

	for i, title := range popularTitles {
		fmt.Printf("%d: %s\n", i+1, title)
		fmt.Printf("Image Path: %s\n", popularImagePaths[i])
	}

	return popularTitles, popularImagePaths
}

func downloadImage(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch image: status code %d", resp.StatusCode)
	}

	// ファイル名をURLから抽出
	fileName := filepath.Base(url)
	filePath := filepath.Join("images", fileName)

	// images ディレクトリが存在しない場合は作成
	err = os.MkdirAll("images", os.ModePerm)
	if err != nil {
		return "", err
	}

	out, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func getComicSummaries(titles []string, imagePaths []string, apiKey string) []entity.Comic {
	client := openai.NewClient(apiKey)

	var mangaData []entity.Comic
	for i, title := range titles {
		req := openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: "あなたは与えられた漫画のタイトルに対してあらすじを提供するように求められました。あらすじの他にも、漫画のジャンル、キャラクター、魅力的な要素、および結論を含めることができます。与えられたタイトルに対して、「Synopsis」「Attraction」「Spoilers」「Genre」「Characters」を決定し、それらをJson形式にして出力してください。なお、Charactersは主要なキャラクター名をカンマ区切りとし、文字列で出力してください。なお、これらは全て固有名詞を除いて日本語で出力してください。",
				},
				{
					Role:    "user",
					Content: title,
				},
			},
			MaxTokens: 1000,
		}

		resp, err := client.CreateChatCompletion(context.Background(), req)
		if err != nil {
			log.Printf("Error calling OpenAI API: %v", err)
			continue
		}

		jsonData := resp.Choices[0].Message.Content
		// jsonDataの先頭に```のような不要な文字が含まれていることがあるため不要な文字を全て削除
		jsonData = strings.ReplaceAll(jsonData, "`", "")

		var tempData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &tempData); err != nil {
			log.Printf("Error unmarshalling JSON data: %v", err)
			continue
		}

		comic := entity.Comic{
			ID:         fmt.Sprintf("%d", i+1),
			Title:      title,
			Synopsis:   getString(tempData, "Synopsis"),
			Attraction: getString(tempData, "Attraction"),
			Spoilers:   getString(tempData, "Spoilers"),
			Genre:      getString(tempData, "Genre"),
			Characters: getString(tempData, "Characters"),
			ImagePath:  imagePaths[i],
		}

		mangaData = append(mangaData, comic)

		fmt.Println(title + "" + jsonData)
	}

	return mangaData
}

func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		return val.(string)
	}
	return ""
}

func storeComicData(mangaData []Comic) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-2"),
		config.WithEndpointResolver(aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				if service == dynamodb.ServiceID && region == "us-west-2" {
					return aws.Endpoint{
						PartitionID:   "aws",
						URL:           "http://localhost:8000",
						SigningRegion: "us-west-2",
					}, nil
				}
				return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
			}),
		),
	)
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}

	svc := dynamodb.NewFromConfig(cfg)

	for _, manga := range mangaData {
		item, err := attributevalue.MarshalMap(manga)
		if err != nil {
			log.Fatalf("Error marshalling item: %v", err)
		}

		_, err = svc.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: aws.String("ComicSummaries"),
			Item:      item,
		})
		if err != nil {
			log.Fatalf("Error putting item into DynamoDB: %v", err)
		}
	}

	fmt.Println("データの追加が完了しました。")
}
