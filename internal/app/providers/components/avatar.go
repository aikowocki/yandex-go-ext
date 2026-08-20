package components

import (
	"fmt"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/infra/postgres"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	avatarusecase "github.com/aikowocki/yandex-go-ext/internal/usecase/avatar"
	imageprocessor "github.com/aikowocki/yandex-go-ext/internal/usecase/image"
	"github.com/aikowocki/yandex-go-ext/internal/worker"
)

// Avatar объединяет зависимости avatar-подсистемы.
type Avatar struct {
	DB            *postgres.DB
	Avatar        *avatarusecase.UseCase
	AvatarRepo    contracts.AvatarRepository
	ThumbnailRepo contracts.ThumbnailRepository
	BlobRepo      contracts.BlobRepository
	Storage       contracts.ObjectStorage
	Processor     contracts.ImageProcessor
	Broker        contracts.MessageBroker
	Outbox        contracts.OutboxRepository
	Worker        *worker.Worker
}

// NewAvatar собирает зависимости avatar-подсистемы.
func NewAvatar(db *postgres.DB, storage contracts.ObjectStorage, messageBroker contracts.MessageBroker, avatarLogger logging.Logger, workerLogger logging.Logger) (*Avatar, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if storage == nil {
		return nil, fmt.Errorf("object storage is nil")
	}
	if messageBroker == nil {
		return nil, fmt.Errorf("message broker is nil")
	}

	avatarRepo := postgres.NewAvatarRepository(db)
	thumbnailRepo := postgres.NewThumbnailRepository(db)
	blobRepo := postgres.NewBlobRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)
	processor := imageprocessor.NewProcessor()
	txManager := postgres.NewTxManager(db)
	avatarUseCase := avatarusecase.New(avatarRepo, thumbnailRepo, storage, messageBroker, processor, outboxRepo, blobRepo, txManager, avatarLogger)

	return &Avatar{
		DB:            db,
		Avatar:        avatarUseCase,
		AvatarRepo:    avatarRepo,
		ThumbnailRepo: thumbnailRepo,
		BlobRepo:      blobRepo,
		Storage:       storage,
		Processor:     processor,
		Broker:        messageBroker,
		Outbox:        outboxRepo,
		Worker:        worker.NewWorker(avatarRepo, thumbnailRepo, storage, processor, messageBroker, txManager, workerLogger, messageBroker, outboxRepo, blobRepo),
	}, nil
}
