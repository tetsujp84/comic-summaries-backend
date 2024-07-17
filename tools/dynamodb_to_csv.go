//go:build dynamodb_to_csv

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/joho/godotenv"
	"log"
	"os"
)

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

	// Scan the DynamoDB table
	items, err := scanDynamoDBTable(svc, "ComicSummaries")
	if err != nil {
		log.Fatalf("Failed to scan table: %v", err)
	}

	// Write the scanned items to a CSV file
	err = writeCSVFile("data.csv", items)
	if err != nil {
		log.Fatalf("Failed to write CSV file: %v", err)
	}

	fmt.Println("Data successfully exported to data.csv")
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

// writeCSVFile writes the given data to a CSV file with the specified filename
func writeCSVFile(filename string, items []map[string]types.AttributeValue) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"ID", "Title", "Synopsis", "Attraction", "Spoilers", "Genre", "Characters", "ImagePath"}
	err = writer.Write(header)
	if err != nil {
		return err
	}

	// Write rows with a unique sequential ID
	id := 1
	for _, item := range items {
		row := []string{
			fmt.Sprintf("%d", id),
			getStringValue(item["Title"]),
			getStringValue(item["Synopsis"]),
			getStringValue(item["Attraction"]),
			getStringValue(item["Spoilers"]),
			getStringValue(item["Genre"]),
			getStringValue(item["Characters"]),
			getStringValue(item["ImagePath"]),
		}
		err = writer.Write(row)
		if err != nil {
			return err
		}
		id++
	}

	return nil
}

// getStringValue returns the string representation of a DynamoDB attribute value
func getStringValue(av types.AttributeValue) string {
	if av == nil {
		return ""
	}
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		return v.Value
	case *types.AttributeValueMemberB:
		return string(v.Value)
	case *types.AttributeValueMemberBOOL:
		return fmt.Sprintf("%v", v.Value)
	case *types.AttributeValueMemberSS:
		return fmt.Sprintf("%v", v.Value)
	case *types.AttributeValueMemberNS:
		return fmt.Sprintf("%v", v.Value)
	case *types.AttributeValueMemberBS:
		return fmt.Sprintf("%v", v.Value)
	case *types.AttributeValueMemberM:
		return fmt.Sprintf("%v", v.Value)
	case *types.AttributeValueMemberL:
		return fmt.Sprintf("%v", v.Value)
	default:
		return ""
	}
}
