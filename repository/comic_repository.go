// repository/comic_repository.go

package repository

import (
	"comic-summaries/entity"
	"context"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type IComicRepository interface {
	FindByID(ctx context.Context, id string) (*entity.Comic, error)
	FindAll(ctx context.Context, limit int, lastEvaluatedKey map[string]*dynamodb.AttributeValue) ([]*entity.Comic, map[string]*dynamodb.AttributeValue, error)
	FindByTitle(ctx context.Context, title string) ([]*entity.Comic, error)
	GetTotalCount(ctx context.Context) (int, error)
}

type comicRepository struct {
	db *dynamodb.DynamoDB
}

func NewComicRepository() IComicRepository {
	dynamodbEndpoint := os.Getenv("DYNAMODB_ENDPOINT")

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(os.Getenv("AWS_REGION")),
		Endpoint:    aws.String(dynamodbEndpoint),
		Credentials: credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
	}))

	db := dynamodb.New(sess)
	return &comicRepository{
		db: db,
	}
}

func (r *comicRepository) FindByID(ctx context.Context, id string) (*entity.Comic, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String("ComicSummaries"),
		Key: map[string]*dynamodb.AttributeValue{
			"ID": {
				S: aws.String(id),
			},
		},
	}
	result, err := r.db.GetItemWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	// データが見つからなかった場合
	if result.Item == nil {
		return nil, nil
	}

	comic := &entity.Comic{}
	if err := dynamodbattribute.UnmarshalMap(result.Item, comic); err != nil {
		return nil, err
	}
	return comic, nil
}

func (r *comicRepository) FindAll(ctx context.Context, limit int, lastEvaluatedKey map[string]*dynamodb.AttributeValue) ([]*entity.Comic, map[string]*dynamodb.AttributeValue, error) {
	input := &dynamodb.ScanInput{
		TableName:         aws.String("ComicSummaries"),
		Limit:             aws.Int64(int64(limit)),
		ExclusiveStartKey: lastEvaluatedKey,
	}

	result, err := r.db.ScanWithContext(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	comics := make([]*entity.Comic, 0)
	for _, item := range result.Items {
		comic := new(entity.Comic)
		if err := dynamodbattribute.UnmarshalMap(item, comic); err != nil {
			return nil, nil, err
		}
		comics = append(comics, comic)
	}

	return comics, result.LastEvaluatedKey, nil
}

func (r *comicRepository) FindByTitle(ctx context.Context, title string) ([]*entity.Comic, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String("ComicSummaries"),
		FilterExpression: aws.String("contains(#title, :title)"),
		ExpressionAttributeNames: map[string]*string{
			"#title": aws.String("Title"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":title": {
				S: aws.String(title),
			},
		},
	}

	result, err := r.db.ScanWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	comics := make([]*entity.Comic, 0)
	for _, item := range result.Items {
		comic := new(entity.Comic)
		if err := dynamodbattribute.UnmarshalMap(item, comic); err != nil {
			return nil, err
		}
		comics = append(comics, comic)
	}

	return comics, nil
}

func (r *comicRepository) GetTotalCount(ctx context.Context) (int, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String("ComicSummaries"),
		Select:    aws.String("COUNT"),
	}

	result, err := r.db.ScanWithContext(ctx, input)
	if err != nil {
		return 0, err
	}

	return int(*result.Count), nil
}
