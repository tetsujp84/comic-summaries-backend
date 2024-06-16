//go:build summary_adder

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
const prompt = "あなたは漫画のタイトルを入力として受け取り、以下の情報を提供します。\n・あらすじ（Synopsis）\n・ジャンル（Genre）\n・登場キャラクター（Characters）\n・魅力的な要素（Attraction）\n・ネタバレ、重要な分岐点や驚きの事実、物語の結末（Spoilers）\nこれらをJson形式にして出力してください。\nJson形式は具体的には以下となります。\n{\n    \"Synopsis\":\"あらすじ\",\n    \"Genre\":\"ジャンル\",\n    \"Characters\":\"キャラクターA、キャラクターB、キャラクターC\",\n    \"Attraction\":\"魅力的な要素\",\n    \"Spoilers\":\"ネタバレ、重要な分岐点や驚きの事実、物語の結末\"\n}\n\n出力にあたってCharactersは主要なキャラクター名をカンマ区切りとし、文字列で出力してください。キャラクターの魅力的な要素がある場合はAttractionに記述し、もしそれが物語における重要な要素であったりネタバレに値する場合はSpoilersに記述するようにしてください。\n文字数に関する制限は以下ですが、自然で惹きつける文章になることを優先し、文字数が制限より前後してしまってもかまいません。\nあらすじは100文字以上で構成してください。\n魅力的な要素は300文字以上で構成してください。\nネタバレ、重要な分岐点や驚きの事実は1000文字以上で構成してください。\nなお、これらは全て固有名詞を除いて日本語で出力してください。"

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	popularTitles, imagePaths := scrapeComicTitlesAndImages("https://comic.k-manga.jp/search/magazine/43", 50)
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

func getComicSummaries(titles []string, imageUrls []string, apiKey string) []entity.Comic {
	client := openai.NewClient(apiKey)

	// S3クライアントの設定
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")))
	if err != nil {
		log.Fatalf("Error loading AWS config: %v", err)
	}
	s3Client := s3.NewFromConfig(cfg)
	bucketName := "comic-summaries"

	var mangaData []entity.Comic
	for i, title := range titles {
		req := openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
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

		imagePath, err := downloadImageAndUploadToS3(imageUrls[i], s3Client, bucketName)
		if err != nil {
			log.Fatalf("Error uploading image to S3: %v", err)
		}

		comic := entity.Comic{
			ID:         fmt.Sprintf("%d", i+1),
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
			TableName: aws.String("ComicSummaries"),
			Item:      item,
		})
		if err != nil {
			log.Fatalf("Error putting item into DynamoDB: %v", err)
		}
	}

	fmt.Println("データの追加が完了しました。")
}
