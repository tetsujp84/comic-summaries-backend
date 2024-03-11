// repository/comic_repository.go

package repository

import (
	"comic-summaries/entity"
	"context"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/joho/godotenv"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type IComicRepository interface {
	FindByID(ctx context.Context, id string) (*entity.Comic, error)
	FindAll(ctx context.Context) ([]*entity.Comic, error)
}

type comicRepository struct {
	db *dynamodb.DynamoDB
}

// NewComicRepository は新しいcomicRepositoryインスタンスを生成します。
func NewComicRepository() IComicRepository {
	// デバッグのため強制終了
	if os.Getenv("GO_ENV") == "dev" {
		err := godotenv.Load()
		if err != nil {
			log.Fatalln(err)
		}
	}
	// 開発時のローカルDBエンドポイント
	dynamodbEndpoint := os.Getenv("DYNAMODB_ENDPOINT")

	sess := session.Must(session.NewSession(&aws.Config{
		Region:   aws.String("ap-northeast-1"),
		Endpoint: aws.String(dynamodbEndpoint),
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

func (r *comicRepository) FindAll(ctx context.Context) ([]*entity.Comic, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String("ComicSummaries"),
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
