package rest

// @title GophProfile API
// @version 1.0.0
// @description Сервис управления аватарами с асинхронной обработкой, S3 хранилищем и развёртыванием в Kubernetes
// @contact.name Поддержка API
// @contact.email support@gophprofile.local
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
//
// @host localhost:8080
// @BasePath /
// @schemes http https
//
// @securityDefinitions.apikey X-User-ID
// @in header
// @name X-User-ID
// @description Заголовок идентификатора пользователя (обязателен для операций записи)
//
// @externalDocs.description OpenAPI документация
// @externalDocs.url https://github.com/aikowocki/yandex-go-ext
