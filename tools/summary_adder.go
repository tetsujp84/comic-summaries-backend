//go:build summary_adder

// 取得してローカルのDBに追加する仕組み
// NOTE:CSV→DynamoDBのほうが汎用性があること、Titleのソートキー化をやめたため正常に追加できなくなっていることから使用しない
// TitleをGSIのキーに使用すべき

package main

import (
	"comic-summaries/entity"
	"context"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const cloudFrontURL = "https://d3pqvcltup9bej.cloudfront.net"

func main() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	prompt, err := os.ReadFile("prompt.txt")
	if err != nil {
		log.Fatalf("Error reading prompt file: %v", err)
	}

	// forでhttps://comic.k-manga.jp/search/magazine/43?search_option%5Bsort%5D=popular&page=1のpageを1から11まで回す
	for i := 1; i < 11; i++ {
		popularTitles, imagePaths := scrapeComicTitlesAndImages("https://comic.k-manga.jp/search/magazine/43?search_option%5Bsort%5D=popular&page="+fmt.Sprintf("%d", i), 50)
		mangaData := getComicSummaries(popularTitles, imagePaths, string(prompt), os.Getenv("OPENAI_API_KEY"))
		storeComicData(mangaData)
	}
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
	var imageUrls []string
	doc.Find(".book-list--item").Each(func(i int, s *goquery.Selection) {
		title := s.Find(".book-list--title").Text()
		imageUrl, exists := s.Find(".book-list--img").Attr("src")
		if exists {
			titles = append(titles, title)
			imageUrls = append(imageUrls, imageUrl)
			fmt.Println(title, imageUrl)
		}
	})

	popularTitles := titles[:count]
	popularImageUrls := imageUrls[:count]

	for i, title := range popularTitles {
		fmt.Printf("%d: %s\n", i+1, title)
		fmt.Printf("Image URL: %s\n", popularImageUrls[i])
	}

	return popularTitles, popularImageUrls
}

func uploadToS3(s3Client *s3.Client, bucketName string, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Generate a unique file name for the S3 object
	key := uuid.New().String() + filepath.Ext(filePath)

	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("image/jpeg"), // Adjust the content type as necessary
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", cloudFrontURL, key), nil
}

func downloadImageAndUploadToS3(url string, s3Client *s3.Client, bucketName string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch image: status code %d", resp.StatusCode)
	}

	// Save the image temporarily to upload to S3
	ext := filepath.Ext(url)
	fileName := uuid.New().String() + ext
	filePath := filepath.Join("temp", fileName)

	// Create temp directory if not exists
	err = os.MkdirAll("temp", os.ModePerm)
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

	// Upload to S3
	s3Url, err := uploadToS3(s3Client, bucketName, filePath)
	if err != nil {
		return "", err
	}

	// Clean up temp file
	err = os.Remove(filePath)
	if err != nil {
		log.Printf("Error removing temp file: %v", err)
	}

	return s3Url, nil
}

func getComicSummaries(titles []string, imageUrls []string, prompt string, apiKey string) []entity.Comic {
	client := openai.NewClient(apiKey)

	// S3クライアントの設定
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}
	s3Client := s3.NewFromConfig(cfg)
	bucketName := "comic-summaries"

	var mangaData []entity.Comic
	// TODO:getComicSummariesを複数実行するとIDが被ってしまう
	for i, title := range titles {
		req := openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    "system",
					Content: prompt,
				},
				{
					Role:    "user",
					Content: title,
				},
			},
			MaxTokens: 4000,
			// ResponseFormatにJSONを指定する
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject, // JSON形式でレスポンスを指定
			},
		}

		resp, err := client.CreateChatCompletion(context.Background(), req)
		if err != nil {
			log.Printf("Error calling OpenAI API: %v", err)
			continue
		}

		jsonData := resp.Choices[0].Message.Content
		// jsonDataの先頭に```のような不要な文字が含まれていることがあるため不要な文字を全て削除
		jsonData = strings.ReplaceAll(jsonData, "`", "")
		log.Printf("output: %s\n%s", title, jsonData)

		var tempData map[string]interface{}
		if err := json.Unmarshal([]byte(jsonData), &tempData); err != nil {
			log.Printf("Error unmarshalling JSON data: %v", err)
			continue
		}

		imagePath, err := downloadImageAndUploadToS3(imageUrls[i], s3Client, bucketName)
		if err != nil {
			log.Fatalf("Error uploading image to S3: %v", err)
		}

		comic := entity.Comic{
			ID:         i + 1,
			Title:      title,
			Synopsis:   getString(tempData, "Synopsis"),
			Attraction: getString(tempData, "Attraction"),
			Spoilers:   getString(tempData, "Spoilers"),
			Genre:      getString(tempData, "Genre"),
			Characters: getString(tempData, "Characters"),
			ImagePath:  imagePath,
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

func storeComicData(mangaData []entity.Comic) {
	awsRegion := os.Getenv("AWS_REGION")
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
		config.WithEndpointResolver(aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				if service == dynamodb.ServiceID && region == awsRegion {
					return aws.Endpoint{
						PartitionID:   "aws",
						URL:           "http://localhost:8000",
						SigningRegion: awsRegion,
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
			TableName:           aws.String("ComicSummaries"),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(ID)"),
		})
		if err != nil {
			// If item exists, update it instead of adding a new item
			_, err = svc.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
				TableName: aws.String("ComicSummaries"),
				Key: map[string]types.AttributeValue{
					"ID":    &types.AttributeValueMemberN{Value: string(manga.ID)},
					"Title": &types.AttributeValueMemberS{Value: manga.Title},
				},
				UpdateExpression: aws.String("set Synopsis = :s, Attraction = :a, Spoilers = :sp, Genre = :g, Characters = :c, ImagePath = :ip"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":s":  &types.AttributeValueMemberS{Value: manga.Synopsis},
					":a":  &types.AttributeValueMemberS{Value: manga.Attraction},
					":sp": &types.AttributeValueMemberS{Value: manga.Spoilers},
					":g":  &types.AttributeValueMemberS{Value: manga.Genre},
					":c":  &types.AttributeValueMemberS{Value: manga.Characters},
					":ip": &types.AttributeValueMemberS{Value: manga.ImagePath},
				},
			})
			if err != nil {
				log.Fatalf("Error updating item in DynamoDB: %v", err)
			}
		}
	}

	fmt.Println("データの追加および更新が完了しました。")
}
