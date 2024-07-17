package main

import (
	"context"
	"fmt"
	"log"
	"os"
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

	// Load the AWS configuration for local DynamoDB
	localCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("AWS_REGION")), config.WithEndpointResolver(
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
		log.Fatalf("Unable to load local SDK config, %v", err)
	}

	// Create DynamoDB client for local DynamoDB
	localSvc := dynamodb.NewFromConfig(localCfg)

	// Load the AWS configuration for remote DynamoDB
	remoteCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(os.Getenv("REMOTE_AWS_REGION")))
	if err != nil {
		log.Fatalf("Unable to load remote SDK config, %v", err)
	}

	// Create DynamoDB client for remote DynamoDB
	remoteSvc := dynamodb.NewFromConfig(remoteCfg)

	// Scan the local DynamoDB table
	items, err := scanDynamoDBTable(localSvc, "ComicSummaries")
	if err != nil {
		log.Fatalf("Failed to scan local table: %v", err)
	}

	// Batch write to remote DynamoDB
	err = batchWriteToDynamoDB(remoteSvc, "ComicSummaries", items)
	if err != nil {
		log.Fatalf("Failed to batch write to remote DynamoDB: %v", err)
	}

	fmt.Println("Data successfully copied to remote DynamoDB")
}

// scanDynamoDBTable scans the specified DynamoDB table and returns all items
func scanDynamoDBTable(svc *dynamodb.Client, tableName string) ([]map[string]types.AttributeValue, error) {
	var items []map[string]types.AttributeValue
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		out, err := svc.Scan(context.TODO(), &dynamodb.ScanInput{
			TableName:         &tableName,
			ExclusiveStartKey: lastEvaluatedKey,
		})
		if err != nil {
			return nil, err
		}

		items = append(items, out.Items...)

		if out.LastEvaluatedKey == nil {
			break
		}
		lastEvaluatedKey = out.LastEvaluatedKey
	}

	return items, nil
}

// batchWriteToDynamoDB writes the records to DynamoDB in batches
func batchWriteToDynamoDB(svc *dynamodb.Client, tableName string, items []map[string]types.AttributeValue) error {
	const batchSize = 10
	const maxRetries = 5
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		writeRequests := make([]types.WriteRequest, 0, batchSize)
		for _, item := range items[i:end] {
			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		// リトライロジックを追加
		for attempt := 1; attempt <= maxRetries; attempt++ {
			_, err := svc.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					tableName: writeRequests,
				},
			})

			if err == nil {
				break
			}

			if dynamodbErr, ok := err.(*types.ProvisionedThroughputExceededException); ok {
				log.Printf("Provisioned throughput exceeded, attempt %d/%d: %v", attempt, maxRetries, dynamodbErr)
				time.Sleep(time.Duration(attempt) * time.Second) // エクスポネンシャルバックオフ
				continue
			}

			return err
		}

		// 進捗状況を表示
		fmt.Printf("Batch write successful %d\n", i)
		time.Sleep(3 * time.Second) // 適切なレート制限を行うためにスリープ
	}

	return nil
}
