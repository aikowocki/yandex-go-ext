package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aikowocki/yandex-go-ext/internal/domain"
	restmiddleware "github.com/aikowocki/yandex-go-ext/internal/transport/rest/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type thumbnailRepositoryStub struct {
	items []*domain.Thumbnail
	err   error
}

func (s thumbnailRepositoryStub) Create(_ context.Context, _ *domain.Thumbnail) error { return nil }
func (s thumbnailRepositoryStub) ListByAvatarID(_ context.Context, _ uuid.UUID) ([]*domain.Thumbnail, error) {
	return s.items, s.err
}
func (s thumbnailRepositoryStub) DeleteByAvatarID(_ context.Context, _ uuid.UUID) error { return nil }

func newRESTContext(target string) echo.Context {
	e := echo.New()
	return e.NewContext(httptest.NewRequest(http.MethodGet, target, nil), httptest.NewRecorder())
}

func TestParseID(t *testing.T) {
	id := uuid.New()
	ctx := newRESTContext("/")
	ctx.SetPath("/avatars/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues(id.String())
	got, err := parseID(ctx)
	if err != nil || got != id {
		t.Fatalf("parseID() = %v, %v", got, err)
	}

	ctx.SetParamValues("not-a-uuid")
	if _, err := parseID(ctx); err != domain.ErrInvalidInput {
		t.Fatalf("invalid id error = %v", err)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name   string
		target string
		limit  int
		offset int
	}{
		{name: "defaults", target: "/", limit: 20, offset: 0},
		{name: "valid values", target: "/?limit=50&offset=4", limit: 50, offset: 4},
		{name: "invalid limit", target: "/?limit=101&offset=-2", limit: 20, offset: 0},
		{name: "invalid numbers", target: "/?limit=no&offset=bad", limit: 20, offset: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := parsePagination(newRESTContext(tt.target))
			if limit != tt.limit || offset != tt.offset {
				t.Fatalf("parsePagination() = %d, %d", limit, offset)
			}
		})
	}
}

func TestContentTypeAndSafeFilename(t *testing.T) {
	for _, tt := range []struct{ key, want string }{
		{"avatar.png", "image/png"},
		{"avatar.WEBP", "image/webp"},
		{"avatar.jpg", "image/jpeg"},
	} {
		if got := contentTypeForKey(tt.key); got != tt.want {
			t.Errorf("contentTypeForKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
	for _, tt := range []struct{ input, want string }{
		{"avatar.jpg", "avatar.jpg"},
		{`a"b
c`, "abc"},
		{"", "avatar"},
	} {
		if got := safeFilename(tt.input); got != tt.want {
			t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseCrop(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target string
		want   domain.AvatarCrop
		err    bool
	}{
		{name: "empty", target: "/", want: domain.AvatarCrop{}},
		{name: "valid", target: "/?crop_x=10.5&crop_y=20&crop_size=300", want: domain.AvatarCrop{X: 10.5, Y: 20, Size: 300}},
		{name: "missing field", target: "/?crop_x=10&crop_y=20", err: true},
		{name: "invalid number", target: "/?crop_x=bad&crop_y=20&crop_size=300", err: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCrop(newRESTContext(tt.target))
			if tt.err {
				if err == nil {
					t.Fatal("invalid crop accepted")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("parseCrop() = %+v, %v", got, err)
			}
		})
	}
}

func TestFindThumbnail(t *testing.T) {
	avatarID := uuid.New()
	wanted := &domain.Thumbnail{ID: uuid.New(), AvatarID: avatarID, Size: domain.ThumbnailSize100x100}
	handler := &avatarHandler{thumbnails: thumbnailRepositoryStub{items: []*domain.Thumbnail{wanted}}}
	got, err := handler.findThumbnail(context.Background(), avatarID, domain.ThumbnailSize100x100)
	if err != nil || got != wanted {
		t.Fatalf("findThumbnail() = %v, %v", got, err)
	}
	if _, err := handler.findThumbnail(context.Background(), avatarID, "bad"); err != domain.ErrInvalidInput {
		t.Fatalf("unsupported size error = %v", err)
	}
	missingHandler := &avatarHandler{thumbnails: thumbnailRepositoryStub{items: nil}}
	if _, err := missingHandler.findThumbnail(context.Background(), avatarID, domain.ThumbnailSize300x300); err != domain.ErrNotFound {
		t.Fatalf("missing thumbnail error = %v", err)
	}
}

func TestRequireUserID(t *testing.T) {
	e := echo.New()
	next := func(c echo.Context) error { return c.String(http.StatusOK, restmiddleware.UserID(c)) }
	handler := restmiddleware.RequireUserID(next)

	for _, tt := range []struct {
		name   string
		header string
		status int
		body   string
	}{
		{name: "missing", status: http.StatusBadRequest},
		{name: "blank", header: "  ", status: http.StatusBadRequest},
		{name: "valid", header: " user-1 ", status: http.StatusOK, body: "user-1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-User-ID", tt.header)
			recorder := httptest.NewRecorder()
			ctx := e.NewContext(req, recorder)
			if err := handler(ctx); err != nil {
				httpErr, ok := err.(*echo.HTTPError)
				if !ok || httpErr.Code != tt.status {
					t.Fatalf("handler error = %v", err)
				}
				return
			}
			if recorder.Code != tt.status || recorder.Body.String() != tt.body {
				t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}
