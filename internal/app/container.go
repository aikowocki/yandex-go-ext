package app

import (
	"sync"

	"github.com/aikowocki/yandex-go-ext/internal/app/providers/components"
	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/transport/rest"
	"github.com/aikowocki/yandex-go-ext/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Container объединяет зависимости и компоненты приложения.
type Container struct {
	Config *config.Config
	DB     *pgxpool.Pool
	Avatar *components.Avatar
	Broker contracts.MessageBroker
	Worker *worker.Worker
	Server *rest.Server

	logger        logging.Logger
	restoreLogger func()
	closeOnce     sync.Once
	closeErr      error
}
