package rest

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/transport/rest/dto"
	restmiddleware "github.com/aikowocki/yandex-go-ext/internal/transport/rest/middleware"
	avatarusecase "github.com/aikowocki/yandex-go-ext/internal/usecase/avatar"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type avatarHandler struct {
	avatar     *avatarusecase.UseCase
	thumbnails contracts.ThumbnailRepository
	storage    contracts.ObjectStorage
}

// @Summary Создание нового аватара
// @Description Загрузка файла изображения для создания нового аватара. Требует multipart/form-data с файлом и опциональными параметрами обрезки.
// @Tags Аватары
// @Accept multipart/form-data
// @Produce json
// @Param X-User-ID header string true "Идентификатор пользователя (обязателен)"
// @Param file formData file true "Файл изображения (JPEG, PNG, WebP)"
// @Param crop_x formData number false "Координата X обрезки (0.0-1.0)"
// @Param crop_y formData number false "Координата Y обрезки (0.0-1.0)"
// @Param crop_size formData number false "Размер обрезки (0.0-1.0)"
// @Success 201 {object} dto.UploadResponse
// @Failure 400 {object} dto.ErrorResponse "Некорректный ввод или неподдерживаемый формат файла"
// @Failure 409 {object} dto.ErrorResponse "Аватар уже существует"
// @Failure 413 {object} dto.ErrorResponse "Файл слишком большой (превышает 10МБ)"
// @Failure 500 {object} dto.ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/avatars [post]
func (h *avatarHandler) createAvatar(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return respondError(c, fmt.Errorf("read multipart file: %w", domain.ErrInvalidInput))
	}
	crop, err := parseCrop(c)
	if err != nil {
		return respondError(c, err)
	}
	avatar, err := h.avatar.Create(c.Request().Context(), restmiddleware.UserID(c), file, crop)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, dto.UploadFromDomain(avatar))
}

// @Summary Обновление параметров обрезки аватара
// @Description Обновляет координаты обрезки существующего аватара. Только владелец может обновлять свой аватар.
// @Tags Аватары
// @Accept json
// @Produce json
// @Param X-User-ID header string true "Идентификатор пользователя (должен совпадать с владельцем)"
// @Param id path string true "UUID аватара"
// @Param body body object true "Параметры обрезки"
// @Success 200 {object} dto.AvatarResponse
// @Failure 400 {object} dto.ErrorResponse "Некорректный ввод или отсутствующие параметры"
// @Failure 403 {object} dto.ErrorResponse "Пользователь не является владельцем аватара"
// @Failure 404 {object} dto.ErrorResponse "Аватар не найден"
// @Router /api/v1/avatars/{id}/crop [patch]
func (h *avatarHandler) updateAvatarCrop(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return respondError(c, err)
	}
	var payload struct {
		CropX    *float64 `json:"crop_x"`
		CropY    *float64 `json:"crop_y"`
		CropSize *float64 `json:"crop_size"`
	}
	if err := c.Bind(&payload); err != nil || payload.CropX == nil || payload.CropY == nil || payload.CropSize == nil {
		return respondError(c, fmt.Errorf("read crop payload: %w", domain.ErrInvalidInput))
	}
	avatar, err := h.avatar.UpdateCrop(c.Request().Context(), restmiddleware.UserID(c), id, domain.AvatarCrop{
		X:    *payload.CropX,
		Y:    *payload.CropY,
		Size: *payload.CropSize,
	})
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, dto.AvatarFromDomain(avatar))
}

// @Summary Получение списка аватаров пользователя
// @Description Получает постраничный список всех аватаров конкретного пользователя.
// @Tags Аватары
// @Produce json
// @Param user_id path string true "Идентификатор пользователя"
// @Param limit query int false "Элементов на странице (1-100, по умолчанию: 20)"
// @Param offset query int false "Смещение для постраничности (по умолчанию: 0)"
// @Success 200 {object} dto.ListAvatarsResponse "Список аватаров с информацией о постраничности"
// @Failure 400 {object} dto.ErrorResponse "Некорректный ввод"
// @Router /api/v1/users/{user_id}/avatars [get]
func (h *avatarHandler) listUserAvatars(c echo.Context) error {
	userID := strings.TrimSpace(c.Param("user_id"))
	if userID == "" {
		return respondError(c, domain.ErrInvalidInput)
	}
	limit, offset := parsePagination(c)
	avatars, err := h.avatar.List(c.Request().Context(), userID, limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	items := make([]dto.AvatarResponse, 0, len(avatars))
	for _, avatar := range avatars {
		items = append(items, dto.AvatarFromDomain(avatar))
	}
	return c.JSON(http.StatusOK, dto.ListAvatarsResponse{Items: items, Limit: limit, Offset: offset})
}

// @Summary Скачивание изображения аватара
// @Description Скачивает аватар в оригинальном или уменьшенном размере. Возвращает бинарный поток с соответствующим Content-Type.
// @Tags Аватары
// @Produce image/jpeg,image/png,image/webp
// @Param id path string true "UUID аватара"
// @Param size query string false "Размер изображения: original, 100x100, или 300x300 (по умолчанию: original)"
// @Success 200 {file} binary "Бинарные данные изображения аватара"
// @Failure 400 {object} dto.ErrorResponse "Некорректный UUID или параметр размера"
// @Failure 404 {object} dto.ErrorResponse "Аватар или миниатюра не найдены"
// @Router /api/v1/avatars/{id} [get]
func (h *avatarHandler) downloadAvatar(c echo.Context) error {
	avatar, err := h.getAvatar(c)
	if err != nil {
		return respondError(c, err)
	}
	return h.streamAvatar(c, avatar)
}

// @Summary Скачивание текущего аватара пользователя
// @Description Скачивает активный аватар конкретного пользователя. Возвращает бинарный поток изображения.
// @Tags Аватары
// @Produce image/jpeg,image/png,image/webp
// @Param user_id path string true "Идентификатор пользователя"
// @Param size query string false "Размер изображения: original, 100x100, или 300x300 (по умолчанию: original)"
// @Success 200 {file} binary "Бинарные данные аватара пользователя"
// @Failure 400 {object} dto.ErrorResponse "Некорректный ввод"
// @Failure 404 {object} dto.ErrorResponse "У пользователя нет активного аватара"
// @Router /api/v1/users/{user_id}/avatar [get]
func (h *avatarHandler) downloadUserAvatar(c echo.Context) error {
	userID := strings.TrimSpace(c.Param("user_id"))
	if userID == "" {
		return respondError(c, domain.ErrInvalidInput)
	}
	avatar, err := h.avatar.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	return h.streamAvatar(c, avatar)
}

func (h *avatarHandler) streamAvatar(c echo.Context, avatar *domain.Avatar) error {
	key := avatar.S3KeyOriginal
	contentType := avatar.MimeType
	if size := strings.TrimSpace(c.QueryParam("size")); size != "" && size != "original" {
		thumbnail, err := h.findThumbnail(c.Request().Context(), avatar.ID, domain.ThumbnailSize(size))
		if err != nil {
			return respondError(c, err)
		}
		key = thumbnail.S3Key
		contentType = contentTypeForKey(key)
	}
	object, err := h.storage.Download(c.Request().Context(), key)
	if err != nil {
		return respondError(c, err)
	}
	defer func() { _ = object.Close() }()
	c.Response().Header().Set(echo.HeaderContentDisposition, `inline; filename="`+safeFilename(avatar.FileName)+`"`)
	return c.Stream(http.StatusOK, contentType, object)
}

// @Summary Получение метаданных аватара
// @Description Получает метаданные конкретного аватара, включая миниатюры с временными ссылками (действительны 15 минут).
// @Tags Аватары
// @Produce json
// @Param id path string true "UUID аватара"
// @Success 200 {object} dto.AvatarResponse
// @Failure 400 {object} dto.ErrorResponse "Некорректный формат UUID"
// @Failure 404 {object} dto.ErrorResponse "Аватар не найден"
// @Router /api/v1/avatars/{id}/metadata [get]
func (h *avatarHandler) metadata(c echo.Context) error {
	avatar, err := h.getAvatar(c)
	if err != nil {
		return respondError(c, err)
	}
	response := dto.AvatarFromDomain(avatar)
	thumbnails, err := h.thumbnails.ListByAvatarID(c.Request().Context(), avatar.ID)
	if err != nil {
		return respondError(c, err)
	}
	response.Thumbnails = make([]dto.ThumbnailResponse, 0, len(thumbnails))
	for _, thumbnail := range thumbnails {
		url, urlErr := h.storage.GetURL(c.Request().Context(), thumbnail.S3Key, 15*time.Minute)
		if urlErr != nil {
			logging.Warn(c.Request().Context(), "failed to create thumbnail URL", logging.Err(urlErr), logging.UUID("thumbnail_id", thumbnail.ID))
		}
		response.Thumbnails = append(response.Thumbnails, dto.ThumbnailFromDomain(thumbnail, url))
	}
	return c.JSON(http.StatusOK, response)
}

// @Summary Удаление аватара
// @Description Удаляет конкретный аватар. Только владелец может удалять свой аватар.
// @Tags Аватары
// @Param X-User-ID header string true "Идентификатор пользователя (должен совпадать с владельцем)"
// @Param id path string true "UUID аватара"
// @Success 204 "Аватар успешно удалён"
// @Failure 400 {object} dto.ErrorResponse "Некорректный формат UUID"
// @Failure 403 {object} dto.ErrorResponse "Пользователь не является владельцем аватара"
// @Failure 404 {object} dto.ErrorResponse "Аватар не найден"
// @Router /api/v1/avatars/{id} [delete]
func (h *avatarHandler) deleteAvatar(c echo.Context) error {
	id, err := parseID(c)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.avatar.Delete(c.Request().Context(), restmiddleware.UserID(c), id); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// @Summary Удаление текущего аватара пользователя
// @Description Удаляет активный аватар пользователя. ID пользователя в пути должен совпадать с заголовком X-User-ID.
// @Tags Аватары
// @Param X-User-ID header string true "Идентификатор пользователя (должен совпадать с path user_id)"
// @Param user_id path string true "Идентификатор пользователя"
// @Success 204 "Аватар пользователя успешно удалён"
// @Failure 400 {object} dto.ErrorResponse "Некорректный ввод"
// @Failure 403 {object} dto.ErrorResponse "Несовпадение ID пользователя или пользователь не владелец"
// @Failure 404 {object} dto.ErrorResponse "У пользователя нет активного аватара"
// @Router /api/v1/users/{user_id}/avatar [delete]
func (h *avatarHandler) deleteUserAvatar(c echo.Context) error {
	pathUserID := strings.TrimSpace(c.Param("user_id"))
	requestUserID := restmiddleware.UserID(c)
	if pathUserID == "" {
		return respondError(c, domain.ErrInvalidInput)
	}
	if pathUserID != requestUserID {
		return respondError(c, domain.ErrForbidden)
	}
	avatar, err := h.avatar.GetByUserID(c.Request().Context(), pathUserID)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.avatar.Delete(c.Request().Context(), requestUserID, avatar.ID); err != nil {
		return respondError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *avatarHandler) getAvatar(c echo.Context) (*domain.Avatar, error) {
	id, err := parseID(c)
	if err != nil {
		return nil, err
	}
	return h.avatar.Get(c.Request().Context(), id)
}

func (h *avatarHandler) findThumbnail(ctx context.Context, avatarID uuid.UUID, size domain.ThumbnailSize) (*domain.Thumbnail, error) {
	if !size.IsSupported() {
		return nil, domain.ErrInvalidInput
	}
	thumbnails, err := h.thumbnails.ListByAvatarID(ctx, avatarID)
	if err != nil {
		return nil, err
	}
	for _, thumbnail := range thumbnails {
		if thumbnail.Size == size {
			return thumbnail, nil
		}
	}
	return nil, domain.ErrNotFound
}

func parseID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return uuid.Nil, domain.ErrInvalidInput
	}
	return id, nil
}

func parsePagination(c echo.Context) (int, int) {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func contentTypeForKey(key string) string {
	key = strings.ToLower(key)
	switch {
	case strings.HasSuffix(key, ".png"):
		return "image/png"
	case strings.HasSuffix(key, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func safeFilename(filename string) string {
	filename = strings.ReplaceAll(filename, `"`, "")
	filename = strings.ReplaceAll(filename, "\r", "")
	filename = strings.ReplaceAll(filename, "\n", "")
	if filename == "" {
		return "avatar"
	}
	return filename
}

func parseCrop(c echo.Context) (domain.AvatarCrop, error) {
	values := []string{strings.TrimSpace(c.FormValue("crop_x")), strings.TrimSpace(c.FormValue("crop_y")), strings.TrimSpace(c.FormValue("crop_size"))}
	if values[0] == "" && values[1] == "" && values[2] == "" {
		return domain.AvatarCrop{}, nil
	}
	crop := domain.AvatarCrop{}
	parsed := []*float64{&crop.X, &crop.Y, &crop.Size}
	for index, value := range values {
		if value == "" {
			return domain.AvatarCrop{}, fmt.Errorf("crop field %d: %w", index, domain.ErrInvalidInput)
		}
		parsedValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return domain.AvatarCrop{}, fmt.Errorf("crop field %d: %w", index, domain.ErrInvalidInput)
		}
		*parsed[index] = parsedValue
	}
	return crop, nil
}
