// usecase/comic_usecase.go

package usecase

import (
	"comic-summaries/entity"
	"comic-summaries/repository"
	"context"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type IComicUsecase interface {
	GetComicByID(ctx context.Context, id string) (*entity.Comic, error)
	GetAllComics(ctx context.Context, page int) ([]*entity.Comic, map[string]*dynamodb.AttributeValue, error)
	SearchComicsByTitle(ctx context.Context, title string) ([]*entity.Comic, error)
	GetTotalCount(ctx context.Context) (int, error)
}

type comicUsecase struct {
	comicRepo repository.IComicRepository
}

// NewComicUsecase は新しいcomicUsecaseインスタンスを生成します。
func NewComicUsecase(repo repository.IComicRepository) IComicUsecase {
	return &comicUsecase{
		comicRepo: repo,
	}
}

func (u *comicUsecase) GetComicByID(ctx context.Context, id string) (*entity.Comic, error) {
	return u.comicRepo.FindByID(ctx, id)
}

func (u *comicUsecase) GetAllComics(ctx context.Context, page int) ([]*entity.Comic, map[string]*dynamodb.AttributeValue, error) {
	limit := 10
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue = nil
	if page > 1 {
		// 前のページの最後のキーを計算して設定する
		for i := 1; i < page; i++ {
			var err error
			_, lastEvaluatedKey, err = u.comicRepo.FindAll(ctx, limit, lastEvaluatedKey)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	return u.comicRepo.FindAll(ctx, limit, lastEvaluatedKey)
}

func (u *comicUsecase) SearchComicsByTitle(ctx context.Context, title string) ([]*entity.Comic, error) {
	return u.comicRepo.FindByTitle(ctx, title)
}

func (u *comicUsecase) GetTotalCount(ctx context.Context) (int, error) {
	return u.comicRepo.GetTotalCount(ctx)
}
