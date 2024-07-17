//go:build batch_write_to_dynamodb

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/joho/godotenv"
)

type Comic struct {
	ID         int    `csv:"ID"`
	Title      string `csv:"Title"`
	Synopsis   string `csv:"Synopsis"`
	Attraction string `csv:"Attraction"`
	Spoilers   string `csv:"Spoilers"`
	Genre      string `csv:"Genre"`
	Characters string `csv:"Characters"`
	ImagePath  string `csv:"ImagePath"`
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")), config.WithEndpointResolver(
		aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			if service == dynamodb.ServiceID {
				return aws.Endpoint{
					URL:           "http://localhost:8000", // DynamoDB Localのエンドポイント
					SigningRegion: region,
				}, nil
			}
			return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
		}),
	))
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}

	// Create DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	// Read CSV file
	records, err := readCSV("data.csv")
	if err != nil {
		log.Fatalf("Failed to read CSV file: %v", err)
	}

	// Batch write to DynamoDB
	err = batchWriteToDynamoDB(svc, "ComicSummaries", records)
	if err != nil {
		log.Fatalf("Failed to batch write to DynamoDB: %v", err)
	}

	fmt.Println("Data successfully imported to DynamoDB")
}

// readCSV reads the CSV file and returns a slice of Comic structs
func readCSV(filename string) ([]Comic, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	// Parse CSV records into Comic structs
	var comics []Comic
	for i, record := range records {
		if i == 0 {
			continue // Skip header
		}
		id, _ := strconv.Atoi(record[0])
		comic := Comic{
			// IDはrecord[0]をintに変換する
			ID:         id,
			Title:      record[1],
			Synopsis:   record[2],
			Attraction: record[3],
			Spoilers:   record[4],
			Genre:      record[5],
			Characters: record[6],
			ImagePath:  record[7],
		}
		comics = append(comics, comic)
	}

	return comics, nil
}

// batchWriteToDynamoDB writes the records to DynamoDB in batches
func batchWriteToDynamoDB(svc *dynamodb.Client, tableName string, records []Comic) error {
	const batchSize = 25
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		writeRequests := make([]types.WriteRequest, 0, batchSize)
		for _, record := range records[i:end] {
			item, err := attributevalue.MarshalMap(record)
			if err != nil {
				return err
			}

			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		_, err := svc.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				tableName: writeRequests,
			},
		})
		if err != nil {
			fmt.Println(err)
			return err
		}

		fmt.Println("Batch write successful", i)
		time.Sleep(1 * time.Second) // 適切なレート制限を行うためにスリープ
	}

	return nil
}
