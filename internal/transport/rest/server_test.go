package rest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	avatarusecase "github.com/aikowocki/yandex-go-ext/internal/usecase/avatar"
	imageprocessor "github.com/aikowocki/yandex-go-ext/internal/usecase/image"
)

func TestRoutesMatchSpecification(t *testing.T) {
	server, err := NewServer(config.ServerConfig{Host: "127.0.0.1", Port: 8080, MaxUploadSize: 10 * 1024 * 1024, RateLimitPerSecond: 10, RateLimitBurst: 20}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]bool{
		"POST /api/v1/avatars":                 false,
		"PATCH /api/v1/avatars/:id/crop":       false,
		"GET /api/v1/avatars/:id":              false,
		"GET /api/v1/avatars/:id/metadata":     false,
		"DELETE /api/v1/avatars/:id":           false,
		"GET /api/v1/users/:user_id/avatar":    false,
		"DELETE /api/v1/users/:user_id/avatar": false,
		"GET /api/v1/users/:user_id/avatars":   false,
		"GET /health":                          false,
		"GET /web/upload":                      false,
		"POST /web/upload":                     false,
		"GET /web/gallery/:user_id":            false,
	}

	for _, route := range server.echo.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	}

	for route, found := range expected {
		if !found {
			t.Errorf("required route is not registered: %s", route)
		}
	}
}
func TestServerRunShutsDownOnContextCancellation(t *testing.T) {
	server, err := NewServer(config.ServerConfig{Host: "127.0.0.1", Port: 0, ReadTimeout: time.Second, WriteTimeout: time.Second, RateLimitPerSecond: 10, RateLimitBurst: 20}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- server.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("server.Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down after context cancellation")
	}
}

func TestServerRunReturnsListenError(t *testing.T) {
	server, err := NewServer(config.ServerConfig{Host: "bad host", Port: 0, RateLimitPerSecond: 10, RateLimitBurst: 20}, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Run(context.Background()); err == nil {
		t.Fatal("invalid listen address accepted")
	}
}

func TestServerRunReturnsInvalidListenAddressError(t *testing.T) {
	server := &Server{http: &http.Server{Addr: "://invalid"}}
	if err := server.Run(context.Background()); err == nil {
		t.Fatal("invalid listen address accepted")
	}
}

type handlerAvatarRepoStub struct {
	avatar  *domain.Avatar
	list    []*domain.Avatar
	getErr  error
	deleted bool
	updated *domain.Avatar
}

func (s *handlerAvatarRepoStub) Create(context.Context, *domain.Avatar) error { return nil }
func (s *handlerAvatarRepoStub) GetByID(context.Context, uuid.UUID) (*domain.Avatar, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.avatar, nil
}
func (s *handlerAvatarRepoStub) GetByUserID(context.Context, string) (*domain.Avatar, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.avatar, nil
}
func (s *handlerAvatarRepoStub) ListByUserID(context.Context, string, int, int) ([]*domain.Avatar, error) {
	if s.list != nil {
		return s.list, nil
	}
	if s.avatar == nil {
		return nil, nil
	}
	return []*domain.Avatar{s.avatar}, nil
}
func (s *handlerAvatarRepoStub) Update(_ context.Context, avatar *domain.Avatar) error {
	s.updated = avatar
	s.avatar = avatar
	return nil
}
func (s *handlerAvatarRepoStub) Delete(context.Context, uuid.UUID) error {
	s.deleted = true
	return nil
}
func (s *handlerAvatarRepoStub) GetPendingForProcessing(context.Context, int) ([]*domain.Avatar, error) {
	return nil, nil
}
func (s *handlerAvatarRepoStub) UpdateProcessingStatus(context.Context, uuid.UUID, domain.ProcessingStatus) error {
	return nil
}

type handlerThumbnailRepoStub struct {
	items []*domain.Thumbnail
	err   error
}

func (s *handlerThumbnailRepoStub) Create(context.Context, *domain.Thumbnail) error { return nil }
func (s *handlerThumbnailRepoStub) ListByAvatarID(context.Context, uuid.UUID) ([]*domain.Thumbnail, error) {
	return s.items, s.err
}
func (s *handlerThumbnailRepoStub) DeleteByAvatarID(context.Context, uuid.UUID) error { return nil }

type handlerStorageStub struct {
	data []byte
}

func (s *handlerStorageStub) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}
func (s *handlerStorageStub) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s *handlerStorageStub) Delete(context.Context, string) error { return nil }
func (s *handlerStorageStub) GetURL(context.Context, string, time.Duration) (string, error) {
	return "https://storage.test/thumb", nil
}

type handlerPublisherStub struct{}

func (handlerPublisherStub) Publish(context.Context, string, *contracts.Message) error { return nil }

type handlerTxStub struct{}

func (handlerTxStub) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func newHandlerForTest(repo *handlerAvatarRepoStub, thumbnails *handlerThumbnailRepoStub, storage *handlerStorageStub) *avatarHandler {
	uc := avatarusecase.New(repo, thumbnails, storage, handlerPublisherStub{}, imageprocessor.NewProcessor(), nil, nil, handlerTxStub{}, nil)
	return &avatarHandler{avatar: uc, thumbnails: thumbnails, storage: storage}
}

func handlerContext(method, target string, body io.Reader) echo.Context {
	e := echo.New()
	return e.NewContext(httptest.NewRequest(method, target, body), httptest.NewRecorder())
}

func setPathParam(ctx echo.Context, path, name, value string) {
	ctx.SetPath(path)
	ctx.SetParamNames(name)
	ctx.SetParamValues(value)
}

func setUserID(ctx echo.Context, userID string) {
	ctx.Set("gophprofile.user_id", userID)
}

func testAvatar() *domain.Avatar {
	return &domain.Avatar{
		ID:               uuid.New(),
		UserID:           "user-1",
		FileName:         "avatar.jpg",
		MimeType:         "image/jpeg",
		Width:            100,
		Height:           100,
		SizeBytes:        10,
		S3KeyOriginal:    "original.jpg",
		UploadStatus:     domain.UploadStatusCompleted,
		ProcessingStatus: domain.ProcessingStatusCompleted,
		IsActive:         true,
	}
}

func TestAvatarHandlersReadPaths(t *testing.T) {
	avatar := testAvatar()
	repo := &handlerAvatarRepoStub{avatar: avatar}
	thumbnails := &handlerThumbnailRepoStub{items: []*domain.Thumbnail{{ID: uuid.New(), AvatarID: avatar.ID, Size: domain.ThumbnailSize100x100, S3Key: "thumb.png"}}}
	storage := &handlerStorageStub{data: []byte("avatar-data")}
	handler := newHandlerForTest(repo, thumbnails, storage)

	t.Run("metadata", func(t *testing.T) {
		ctx := handlerContext(http.MethodGet, "/", nil)
		setPathParam(ctx, "/avatars/:id/metadata", "id", avatar.ID.String())
		if err := handler.metadata(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK {
			t.Fatalf("metadata status = %d", ctx.Response().Status)
		}
	})
	t.Run("download original", func(t *testing.T) {
		ctx := handlerContext(http.MethodGet, "/", nil)
		setPathParam(ctx, "/avatars/:id", "id", avatar.ID.String())
		if err := handler.downloadAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK || ctx.Response().Header().Get(echo.HeaderContentDisposition) == "" {
			t.Fatalf("download response status=%d headers=%v", ctx.Response().Status, ctx.Response().Header())
		}
	})
	t.Run("download thumbnail", func(t *testing.T) {
		ctx := handlerContext(http.MethodGet, "/?size=100x100", nil)
		setPathParam(ctx, "/avatars/:id", "id", avatar.ID.String())
		if err := handler.downloadAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK {
			t.Fatalf("thumbnail status = %d", ctx.Response().Status)
		}
	})
	t.Run("download by user", func(t *testing.T) {
		ctx := handlerContext(http.MethodGet, "/", nil)
		setPathParam(ctx, "/users/:user_id/avatar", "user_id", "user-1")
		if err := handler.downloadUserAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK {
			t.Fatalf("user download status = %d", ctx.Response().Status)
		}
	})
	t.Run("list", func(t *testing.T) {
		ctx := handlerContext(http.MethodGet, "/?limit=10&offset=2", nil)
		setPathParam(ctx, "/users/:user_id/avatars", "user_id", "user-1")
		if err := handler.listUserAvatars(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK {
			t.Fatalf("list status = %d", ctx.Response().Status)
		}
	})
}

func TestAvatarHandlersMutations(t *testing.T) {
	avatar := testAvatar()
	repo := &handlerAvatarRepoStub{avatar: avatar, list: []*domain.Avatar{avatar}}
	thumbnails := &handlerThumbnailRepoStub{}
	storage := &handlerStorageStub{data: []byte("avatar-data")}
	handler := newHandlerForTest(repo, thumbnails, storage)

	t.Run("update crop", func(t *testing.T) {
		body := bytes.NewBufferString(`{"crop_x":10,"crop_y":10,"crop_size":50}`)
		ctx := handlerContext(http.MethodPatch, "/", body)
		ctx.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		setPathParam(ctx, "/avatars/:id/crop", "id", avatar.ID.String())
		setUserID(ctx, "user-1")
		if err := handler.updateAvatarCrop(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusOK || repo.updated == nil {
			t.Fatalf("crop response status=%d updated=%+v", ctx.Response().Status, repo.updated)
		}
	})
	t.Run("update crop invalid payload", func(t *testing.T) {
		ctx := handlerContext(http.MethodPatch, "/", bytes.NewBufferString(`{"crop_x":10}`))
		ctx.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		setPathParam(ctx, "/avatars/:id/crop", "id", avatar.ID.String())
		setUserID(ctx, "user-1")
		if err := handler.updateAvatarCrop(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusBadRequest {
			t.Fatalf("invalid crop status = %d", ctx.Response().Status)
		}
	})
	t.Run("delete avatar", func(t *testing.T) {
		ctx := handlerContext(http.MethodDelete, "/", nil)
		setPathParam(ctx, "/avatars/:id", "id", avatar.ID.String())
		setUserID(ctx, "user-1")
		if err := handler.deleteAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusNoContent || !repo.deleted {
			t.Fatalf("delete status=%d deleted=%v", ctx.Response().Status, repo.deleted)
		}
	})
	t.Run("delete user avatar forbidden", func(t *testing.T) {
		ctx := handlerContext(http.MethodDelete, "/", nil)
		setPathParam(ctx, "/users/:user_id/avatar", "user_id", "other-user")
		setUserID(ctx, "user-1")
		if err := handler.deleteUserAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusForbidden {
			t.Fatalf("forbidden status = %d", ctx.Response().Status)
		}
	})
	t.Run("delete user avatar", func(t *testing.T) {
		ctx := handlerContext(http.MethodDelete, "/", nil)
		setPathParam(ctx, "/users/:user_id/avatar", "user_id", "user-1")
		setUserID(ctx, "user-1")
		if err := handler.deleteUserAvatar(ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.Response().Status != http.StatusNoContent {
			t.Fatalf("delete user status = %d", ctx.Response().Status)
		}
	})
}

func TestAvatarHandlersValidationErrors(t *testing.T) {
	avatar := testAvatar()
	handler := newHandlerForTest(&handlerAvatarRepoStub{avatar: avatar}, &handlerThumbnailRepoStub{}, &handlerStorageStub{})
	ctx := handlerContext(http.MethodPost, "/", nil)
	setUserID(ctx, "user-1")
	if err := handler.createAvatar(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Response().Status != http.StatusBadRequest {
		t.Fatalf("missing file status = %d", ctx.Response().Status)
	}

	ctx = handlerContext(http.MethodGet, "/", nil)
	setPathParam(ctx, "/avatars/:id", "id", "bad")
	if err := handler.downloadAvatar(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Response().Status != http.StatusBadRequest {
		t.Fatalf("invalid id status = %d", ctx.Response().Status)
	}

	ctx = handlerContext(http.MethodGet, "/", nil)
	setPathParam(ctx, "/users/:user_id/avatars", "user_id", " ")
	if err := handler.listUserAvatars(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Response().Status != http.StatusBadRequest {
		t.Fatalf("invalid user status = %d", ctx.Response().Status)
	}
}
