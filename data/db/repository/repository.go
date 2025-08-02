package repository

import (
	"context"
	"errors"

	"github.com/glekoz/online-shop_image/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	q    *Queries
	pool *pgxpool.Pool
}

func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	queries := New(pool)
	return &Repository{q: queries, pool: pool}, nil
}

func (r *Repository) AddImage(ctx context.Context, image models.EntityImage) error {
	return r.q.AddImage(ctx, AddImageParams(image))
}

func (r *Repository) DeleteImage(ctx context.Context, imagePath string) error {
	if imagePath == "" {
		return models.ErrInvalidInput
	}
	return r.q.DeleteImage(ctx, imagePath)
}

func (r *Repository) CreateEntity(ctx context.Context, service, entityID, status string, maxCount int) error {
	params := CreateEntityParams{
		Service:  service,
		EntityID: entityID,
		Status:   status,
		MaxCount: int32(maxCount),
	}
	return r.q.CreateEntity(ctx, params)
}

func (r *Repository) DeleteEntity(ctx context.Context, service, entityID string) error {
	params := DeleteEntityParams{
		Service:  service,
		EntityID: entityID,
	}
	return r.q.DeleteEntity(ctx, params)
}

func (r *Repository) GetEntityState(ctx context.Context, service, entityID string) (models.EntityState, error) {
	params := GetEntityStateParams{
		Service:  service,
		EntityID: entityID,
	}
	state, err := r.q.GetEntityState(ctx, params)
	if err != nil {
		return models.EntityState{}, err
	}

	return models.EntityState{
		Service:    state.Service,
		EntityID:   state.EntityID,
		ImageCount: int(state.ImageCount),
		Status:     state.Status,
		MaxCount:   int(state.MaxCount),
	}, nil
}

func (r *Repository) GetImageCover(ctx context.Context, service, entityID string) (models.EntityImage, error) {
	params := GetImageCoverParams{
		Service:  service,
		EntityID: entityID,
	}
	image, err := r.q.GetImageCover(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.EntityImage{}, models.ErrNoRows
		}
		return models.EntityImage{}, err
	}

	return models.EntityImage(image), nil
}

func (r *Repository) GetImageList(ctx context.Context, service, entityID string) ([]models.EntityImage, error) {
	params := GetImageListParams{
		Service:  service,
		EntityID: entityID,
	}
	dbImages, err := r.q.GetImageList(ctx, params)
	if err != nil {
		return nil, err
	}
	var images []models.EntityImage
	for _, image := range dbImages {
		images = append(images, models.EntityImage(image))
	}
	return images, nil
}

func (r *Repository) SetBusyStatus(ctx context.Context, service, entityID, status string) error {
	params := SetBusyStatusParams{
		Status:   status,
		Service:  service,
		EntityID: entityID,
	}
	err := r.q.SetBusyStatus(ctx, params)
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) SetCountAndFreeStatus(ctx context.Context, service, entityID, status string, images int) error {
	params := SetCountAndFreeStatusParams{
		ImageCount: int32(images),
		Status:     status,
		Service:    service,
		EntityID:   entityID,
	}
	return r.q.SetCountAndFreeStatus(ctx, params)
}
