// usecase/comic_usecase.go

package usecase

import (
	"comic-summaries/entity"
	"comic-summaries/repository"
	"context"
)

type IComicUsecase interface {
	GetComicByID(ctx context.Context, id string) (*entity.Comic, error)
	GetAllComics(ctx context.Context) ([]*entity.Comic, error)
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

func (u *comicUsecase) GetAllComics(ctx context.Context) ([]*entity.Comic, error) {
	return u.comicRepo.FindAll(ctx)
}
