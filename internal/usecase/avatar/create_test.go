package avatar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/domain/events"
	"github.com/google/uuid"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	imageprocessor "github.com/aikowocki/yandex-go-ext/internal/usecase/image"
)

type avatarRepoStub struct {
	avatar        *domain.Avatar
	dedupAvatar   *domain.Avatar
	createCalls   int
	activateCalls int
}

func (r *avatarRepoStub) Create(_ context.Context, avatar *domain.Avatar) error {
	r.createCalls++
	r.avatar = cloneAvatar(avatar)
	return nil
}
func (r *avatarRepoStub) GetByID(_ context.Context, _ uuid.UUID) (*domain.Avatar, error) {
	return r.avatar, nil
}
func (r *avatarRepoStub) GetByUserID(_ context.Context, _ string) (*domain.Avatar, error) {
	return r.avatar, nil
}
func (r *avatarRepoStub) ListByUserID(_ context.Context, _ string, _, _ int) ([]*domain.Avatar, error) {
	if r.avatar == nil {
		return nil, nil
	}
	return []*domain.Avatar{r.avatar}, nil
}

func (r *avatarRepoStub) FindByUserAndSourceBlobID(_ context.Context, _ string, _ uuid.UUID) (*domain.Avatar, error) {
	if r.dedupAvatar == nil {
		return nil, domain.ErrNotFound
	}
	return cloneAvatar(r.dedupAvatar), nil
}
func (r *avatarRepoStub) Activate(_ context.Context, _ string, avatarID uuid.UUID) error {
	r.activateCalls++
	if r.dedupAvatar != nil && r.dedupAvatar.ID == avatarID {
		r.dedupAvatar.DeletedAt = nil
		r.dedupAvatar.IsActive = true
	}
	return nil
}
func (r *avatarRepoStub) Update(_ context.Context, avatar *domain.Avatar) error {
	r.avatar = cloneAvatar(avatar)
	return nil
}
func (r *avatarRepoStub) Delete(_ context.Context, _ uuid.UUID) error { return nil }
func (r *avatarRepoStub) GetPendingForProcessing(_ context.Context, _ int) ([]*domain.Avatar, error) {
	return nil, nil
}
func (r *avatarRepoStub) UpdateProcessingStatus(_ context.Context, _ uuid.UUID, _ domain.ProcessingStatus) error {
	return nil
}

func cloneAvatar(avatar *domain.Avatar) *domain.Avatar {
	copy := *avatar
	return &copy
}

type storageStub struct {
	key  string
	data []byte
}

func (s *storageStub) Upload(_ context.Context, key string, data io.Reader, _ int64, _ string) error {
	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	s.key, s.data = key, content
	return nil
}
func (s *storageStub) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}
func (s *storageStub) Delete(_ context.Context, _ string) error { return nil }
func (s *storageStub) GetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + key, nil
}

type publisherStub struct{ message *contracts.Message }

func (p *publisherStub) Publish(_ context.Context, topic string, message *contracts.Message) error {
	message.Topic = topic
	p.message = message
	return nil
}

func multipartFileHeader(t *testing.T, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="avatar.jpg"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return request.MultipartForm.File["file"][0]
}

func TestUseCaseCreateUsesSameIDInStorageKey(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var content bytes.Buffer
	if err := jpeg.Encode(&content, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	repo := &avatarRepoStub{}
	storage := &storageStub{}
	publisher := &publisherStub{}
	useCase := New(repo, nil, storage, publisher, imageprocessor.NewProcessor(), nil)
	avatar, err := useCase.Create(context.Background(), "user-1", multipartFileHeader(t, content.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if avatar.ID == uuid.Nil || !strings.Contains(storage.key, avatar.ID.String()) {
		t.Fatalf("storage key %q does not contain avatar id %s", storage.key, avatar.ID)
	}
	if avatar.UploadStatus != domain.UploadStatusCompleted {
		t.Fatalf("unexpected upload status: %s", avatar.UploadStatus)
	}
	var event events.AvatarUploadedEvent
	if err := json.Unmarshal(publisher.message.Payload, &event); err != nil {
		t.Fatal(err)
	}
	if event.AvatarID != avatar.ID.String() {
		t.Fatal("published event contains a different avatar id")
	}
}

func TestCreateAvatarRejectsOversizedFile(t *testing.T) {
	repo := &avatarRepoStub{}
	useCase := New(repo, nil, &storageStub{}, &publisherStub{}, imageprocessor.NewProcessor(), nil)
	file := &multipart.FileHeader{Filename: "large.jpg", Size: maxAvatarSize + 1, Header: make(textproto.MIMEHeader)}
	file.Header.Set("Content-Type", "image/jpeg")
	_, err := useCase.Create(context.Background(), "user-1", file)
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

type eventPublisherStub struct{ err error }

func (s eventPublisherStub) Publish(context.Context, string, *contracts.Message) error { return s.err }

type eventOutboxStub struct {
	saveErr       error
	markPubErr    error
	markFailed    bool
	markPublished bool
}

func (s *eventOutboxStub) Save(context.Context, *contracts.OutboxEvent) error { return s.saveErr }
func (s *eventOutboxStub) ClaimPending(context.Context, int, time.Duration) ([]*contracts.OutboxEvent, error) {
	return nil, nil
}
func (s *eventOutboxStub) ListPending(context.Context, int) ([]*contracts.OutboxEvent, error) {
	return nil, nil
}
func (s *eventOutboxStub) MarkPublished(context.Context, string) error {
	s.markPublished = true
	return s.markPubErr
}
func (s *eventOutboxStub) MarkFailed(context.Context, string, string, time.Time) error {
	s.markFailed = true
	return nil
}

func TestPublishEventOutboxBranches(t *testing.T) {
	ctx := context.Background()
	if err := publishEvent(ctx, eventPublisherStub{}, nil, "topic", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if err := publishEvent(ctx, eventPublisherStub{err: errors.New("publish")}, nil, "topic", []byte("{}")); err == nil {
		t.Fatal("publisher error was ignored without outbox")
	}
	saveError := &eventOutboxStub{saveErr: errors.New("save")}
	if err := publishEvent(ctx, eventPublisherStub{}, saveError, "topic", []byte("{}")); err == nil {
		t.Fatal("outbox save error was ignored")
	}
	failed := &eventOutboxStub{}
	if err := publishEvent(ctx, eventPublisherStub{err: errors.New("publish")}, failed, "topic", []byte("{}")); err != nil || !failed.markFailed {
		t.Fatalf("publish failure state: err=%v failed=%v", err, failed.markFailed)
	}
	published := &eventOutboxStub{}
	if err := publishEvent(ctx, eventPublisherStub{}, published, "topic", []byte("{}")); err != nil || !published.markPublished {
		t.Fatalf("publish success state: err=%v published=%v", err, published.markPublished)
	}
	if _, err := marshalEvent(func() {}); err == nil {
		t.Fatal("unsupported event was marshaled")
	}
}
func avatarFileHeader(size int64, mime string) *multipart.FileHeader {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", mime)
	return &multipart.FileHeader{Filename: "avatar.jpg", Size: size, Header: header}
}

func TestValidateFileHeader(t *testing.T) {
	tests := []struct {
		name string
		file *multipart.FileHeader
		want error
	}{
		{name: "nil", want: domain.ErrInvalidInput},
		{name: "empty", file: avatarFileHeader(0, "image/jpeg"), want: domain.ErrInvalidFormat},
		{name: "too large", file: avatarFileHeader(maxAvatarSize+1, "image/jpeg"), want: domain.ErrFileTooLarge},
		{name: "unsupported mime", file: avatarFileHeader(10, "text/plain"), want: domain.ErrInvalidFormat},
		{name: "jpeg", file: avatarFileHeader(10, "image/jpeg")},
		{name: "jpg", file: avatarFileHeader(10, "image/jpg")},
		{name: "png", file: avatarFileHeader(10, "image/png")},
		{name: "webp", file: avatarFileHeader(10, "image/webp")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileHeader(tt.file)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("validateFileHeader() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("validateFileHeader() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestFormatAndMimeHelpers(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{" image/jpeg ", "jpeg"},
		{"image/jpg", "jpeg"},
		{"image/png", "png"},
		{"image/webp", "webp"},
		{"image/gif", ""},
	} {
		if got := formatForMimeType(tt.input); got != tt.want {
			t.Errorf("formatForMimeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
	for _, tt := range []struct{ input, want string }{{"jpg", "image/jpeg"}, {"png", "image/png"}, {"webp", "image/webp"}} {
		if got := mimeTypeForFormat(tt.input); got != tt.want {
			t.Errorf("mimeTypeForFormat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeCrop(t *testing.T) {
	got, err := normalizeCrop(domain.AvatarCrop{}, 1200, 800)
	if err != nil || got != (domain.AvatarCrop{X: 200, Y: 0, Size: 800}) {
		t.Fatalf("default crop = %+v, %v", got, err)
	}
	if _, err := normalizeCrop(domain.AvatarCrop{}, 0, 100); !errors.Is(err, domain.ErrInvalidFormat) {
		t.Fatalf("invalid dimensions error = %v", err)
	}
	invalid := []domain.AvatarCrop{
		{X: math.NaN(), Size: 10},
		{Y: math.Inf(1), Size: 10},
		{Size: math.Inf(1)},
		{X: -1, Size: 10},
		{Y: -1, Size: 10},
		{Size: -1},
		{Size: 121},
		{X: 100, Size: 30},
		{Y: 100, Size: 30},
	}
	for _, crop := range invalid {
		if _, err := normalizeCrop(crop, 120, 100); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("crop %+v error = %v", crop, err)
		}
	}
	valid := domain.AvatarCrop{X: 10, Y: 20, Size: 50}
	got, err = normalizeCrop(valid, 120, 100)
	if err != nil || got != valid {
		t.Fatalf("valid crop = %+v, %v", got, err)
	}
}

func TestHashSource(t *testing.T) {
	got, err := hashSource(bytes.NewReader([]byte("avatar")))
	if err != nil || len(got) != 32 {
		t.Fatalf("hashSource() = %x, %v", got, err)
	}
}

func TestUseCaseReadOperations(t *testing.T) {
	id := uuid.New()
	repo := &avatarRepoStub{avatar: &domain.Avatar{ID: id, UserID: "user-1"}}
	uc := New(repo, nil, nil, nil, nil, nil)
	if got, err := uc.Get(context.Background(), id); err != nil || got.ID != id {
		t.Fatalf("Get() = %+v, %v", got, err)
	}
	if _, err := uc.Get(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid Get() error = %v", err)
	}
	if got, err := uc.GetByUserID(context.Background(), "user-1"); err != nil || got.UserID != "user-1" {
		t.Fatalf("GetByUserID() = %+v, %v", got, err)
	}
	if _, err := uc.GetByUserID(context.Background(), " "); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid GetByUserID() error = %v", err)
	}
	if got, err := uc.List(context.Background(), "user-1", 20, 0); err != nil || len(got) != 1 {
		t.Fatalf("List() = %+v, %v", got, err)
	}
	if _, err := uc.List(context.Background(), "", 20, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid List() error = %v", err)
	}
}

type createProcessorStub struct {
	decoded   image.Image
	format    string
	decodeErr error
	magicErr  error
}

func (p createProcessorStub) Decode(io.Reader) (image.Image, string, error) {
	return p.decoded, p.format, p.decodeErr
}
func (p createProcessorStub) CreateThumbnail(image.Image, int, int) image.Image { return nil }
func (p createProcessorStub) Encode(io.Writer, image.Image, string, int) error  { return nil }
func (p createProcessorStub) ValidateMagicBytes([]byte) (string, error) {
	if p.magicErr != nil {
		return "", p.magicErr
	}
	return "jpeg", nil
}

type createFailStorage struct{ err error }

func (s createFailStorage) Upload(context.Context, string, io.Reader, int64, string) error {
	return s.err
}
func (createFailStorage) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (createFailStorage) Delete(context.Context, string) error { return nil }
func (createFailStorage) GetURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

var _ contracts.ImageProcessor = createProcessorStub{}

func TestCreateRejectsValidationAndDecodeErrors(t *testing.T) {
	validContent := []byte("jpeg content")
	validImage := image.NewRGBA(image.Rect(0, 0, 20, 20))
	tests := []struct {
		name      string
		userID    string
		file      *multipart.FileHeader
		processor createProcessorStub
		crop      domain.AvatarCrop
		want      error
	}{
		{name: "empty user", userID: " ", want: domain.ErrInvalidInput},
		{name: "nil file", userID: "user-1", want: domain.ErrInvalidInput},
		{name: "decode error", userID: "user-1", file: multipartFileHeader(t, validContent), processor: createProcessorStub{decoded: validImage, format: "jpeg", decodeErr: errors.New("decode")}},
		{name: "format mismatch", userID: "user-1", file: multipartFileHeader(t, validContent), processor: createProcessorStub{decoded: validImage, format: "png"}, want: domain.ErrInvalidFormat},
		{name: "zero dimensions", userID: "user-1", file: multipartFileHeader(t, validContent), processor: createProcessorStub{decoded: image.NewRGBA(image.Rect(0, 0, 0, 20)), format: "jpeg"}, want: domain.ErrInvalidFormat},
		{name: "invalid crop", userID: "user-1", file: multipartFileHeader(t, validContent), processor: createProcessorStub{decoded: validImage, format: "jpeg"}, crop: domain.AvatarCrop{X: 100, Y: 0, Size: 30}, want: domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := New(&avatarRepoStub{}, nil, &storageStub{}, &publisherStub{}, tt.processor, nil)
			_, err := uc.Create(context.Background(), tt.userID, tt.file, tt.crop)
			if err == nil {
				t.Fatal("invalid create request accepted")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCreateMarksUploadFailure(t *testing.T) {
	file := multipartFileHeader(t, []byte("jpeg content"))
	processor := createProcessorStub{decoded: image.NewRGBA(image.Rect(0, 0, 20, 20)), format: "jpeg"}
	uc := New(&avatarRepoStub{}, nil, createFailStorage{err: errors.New("storage down")}, &publisherStub{}, processor, nil)
	_, err := uc.Create(context.Background(), "user-1", file)
	if err == nil || !strings.Contains(err.Error(), "upload original") {
		t.Fatalf("upload error = %v", err)
	}
}
