package worker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/domain/events"
	"github.com/google/uuid"
	"image"
	"io"
	"strings"
	"testing"
	"time"
)

func TestIsFinalDelivery(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{name: "missing metadata is final", want: true},
		{
			name: "intermediate delivery",
			headers: map[string]string{
				contracts.DeliveryAttemptHeader:     "2",
				contracts.MaxDeliveryAttemptsHeader: "5",
			},
			want: false,
		},
		{
			name: "last delivery",
			headers: map[string]string{
				contracts.DeliveryAttemptHeader:     "5",
				contracts.MaxDeliveryAttemptsHeader: "5",
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message := contracts.NewMessage("avatar.uploaded", nil)
			message.Headers = test.headers
			if got := isFinalDelivery(message); got != test.want {
				t.Fatalf("isFinalDelivery() = %v, want %v", got, test.want)
			}
		})
	}
}

type workerProcessorStub struct {
	decodeImage  image.Image
	decodeFormat string
	decodeErr    error
	encodeErr    error
}

func (s workerProcessorStub) Decode(io.Reader) (image.Image, string, error) {
	return s.decodeImage, s.decodeFormat, s.decodeErr
}
func (workerProcessorStub) CreateThumbnail(img image.Image, _, _ int) image.Image { return img }
func (s workerProcessorStub) Encode(w io.Writer, _ image.Image, _ string, _ int) error {
	if s.encodeErr != nil {
		return s.encodeErr
	}
	_, err := w.Write([]byte("encoded"))
	return err
}
func (workerProcessorStub) ValidateMagicBytes([]byte) (string, error) { return "jpeg", nil }

type workerStorageStub struct {
	uploadErr   error
	deleteErr   error
	downloadErr error
	uploaded    []string
	deleted     []string
}

func (s *workerStorageStub) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) error {
	s.uploaded = append(s.uploaded, key)
	return s.uploadErr
}
func (s *workerStorageStub) Download(context.Context, string) (io.ReadCloser, error) {
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	return io.NopCloser(strings.NewReader("original")), nil
}
func (s *workerStorageStub) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErr
}
func (*workerStorageStub) GetURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type workerThumbnailRepoStub struct {
	createErr error
	created   *domain.Thumbnail
}

func (s *workerThumbnailRepoStub) Create(_ context.Context, thumbnail *domain.Thumbnail) error {
	s.created = thumbnail
	return s.createErr
}
func (*workerThumbnailRepoStub) ListByAvatarID(context.Context, uuid.UUID) ([]*domain.Thumbnail, error) {
	return nil, nil
}
func (*workerThumbnailRepoStub) DeleteByAvatarID(context.Context, uuid.UUID) error { return nil }

type workerBlobRepoStub struct{}

func (workerBlobRepoStub) GetOrCreate(context.Context, *domain.Blob) (*domain.Blob, error) {
	return nil, nil
}
func (workerBlobRepoStub) MarkReady(context.Context, uuid.UUID) error          { return nil }
func (workerBlobRepoStub) MarkFailed(context.Context, uuid.UUID, string) error { return nil }
func (workerBlobRepoStub) EnsureDerivation(context.Context, *domain.BlobDerivation) (*domain.BlobDerivation, error) {
	return nil, nil
}
func (workerBlobRepoStub) LinkThumbnail(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func TestThumbnailSizes(t *testing.T) {
	for _, tt := range []struct {
		name string
		ops  []events.ProcessingOperation
		want []domain.ThumbnailSize
	}{
		{name: "default", want: domain.SupportedThumbnailSizes},
		{name: "requested", ops: []events.ProcessingOperation{events.ProcessingOperationThumbnail300}, want: []domain.ThumbnailSize{domain.ThumbnailSize300x300}},
		{name: "unknown falls back", ops: []events.ProcessingOperation{"unknown"}, want: domain.SupportedThumbnailSizes},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := thumbnailSizes(tt.ops); !sameThumbnailSizes(got, tt.want) {
				t.Fatalf("thumbnailSizes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func sameThumbnailSizes(left, right []domain.ThumbnailSize) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func TestApplyAvatarCropHandlesInvalidAndClampedValues(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	for _, tt := range []struct {
		name string
		crop domain.AvatarCrop
		want image.Rectangle
	}{
		{name: "nil image", crop: domain.AvatarCrop{Size: 10}, want: image.Rectangle{}},
		{name: "invalid size", crop: domain.AvatarCrop{Size: 0}, want: img.Bounds()},
		{name: "negative origin", crop: domain.AvatarCrop{X: -1, Y: 0, Size: 20}, want: img.Bounds()},
		{name: "clamped origin", crop: domain.AvatarCrop{X: 90, Y: 70, Size: 20}, want: image.Rect(0, 0, 20, 20)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var source image.Image = img
			if tt.name == "nil image" {
				source = nil
			}
			got := applyAvatarCrop(source, tt.crop)
			if got == nil {
				if tt.want != (image.Rectangle{}) {
					t.Fatal("unexpected nil crop result")
				}
				return
			}
			if got.Bounds() != tt.want {
				t.Fatalf("crop bounds = %v, want %v", got.Bounds(), tt.want)
			}
		})
	}
}

func TestCreateThumbnailSuccessAndErrors(t *testing.T) {
	avatar := &domain.Avatar{ID: uuid.New(), UserID: "user-1", SourceBlobID: uuid.Nil}
	base := func() (*Worker, *workerStorageStub, *workerThumbnailRepoStub) {
		storage := &workerStorageStub{}
		repo := &workerThumbnailRepoStub{}
		return &Worker{storage: storage, processor: workerProcessorStub{decodeImage: image.NewRGBA(image.Rect(0, 0, 20, 20))}, thumbnailRepo: repo}, storage, repo
	}

	t.Run("success", func(t *testing.T) {
		worker, storage, repo := base()
		if err := worker.createThumbnail(context.Background(), avatar, image.NewRGBA(image.Rect(0, 0, 20, 20)), domain.ThumbnailSize100x100, "PNG"); err != nil {
			t.Fatal(err)
		}
		if len(storage.uploaded) != 1 || repo.created == nil || repo.created.Width != 100 || repo.created.Height != 100 {
			t.Fatalf("thumbnail was not persisted: uploads=%v record=%+v", storage.uploaded, repo.created)
		}
	})

	tests := []struct {
		name      string
		configure func(*Worker, *workerStorageStub, *workerThumbnailRepoStub)
		want      string
	}{
		{name: "unsupported size", want: "unsupported thumbnail size"},
		{name: "nil thumbnail", configure: func(w *Worker, _ *workerStorageStub, _ *workerThumbnailRepoStub) {
			w.processor = nilThumbnailProcessor{}
		}, want: "processor returned nil thumbnail"},
		{name: "encode error", configure: func(w *Worker, _ *workerStorageStub, _ *workerThumbnailRepoStub) {
			w.processor = workerProcessorStub{encodeErr: errors.New("encode")}
		}, want: "encode thumbnail"},
		{name: "upload error", configure: func(_ *Worker, s *workerStorageStub, _ *workerThumbnailRepoStub) {
			s.uploadErr = errors.New("upload")
		}, want: "upload thumbnail"},
		{name: "repository error", configure: func(_ *Worker, _ *workerStorageStub, r *workerThumbnailRepoStub) {
			r.createErr = errors.New("repository")
		}, want: "save thumbnail record"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker, storage, repo := base()
			if tt.configure != nil {
				tt.configure(worker, storage, repo)
			}
			size := domain.ThumbnailSize100x100
			if tt.name == "unsupported size" {
				size = "bad"
			}
			err := worker.createThumbnail(context.Background(), avatar, image.NewRGBA(image.Rect(0, 0, 20, 20)), size, "jpeg")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

type nilThumbnailProcessor struct{}

func (nilThumbnailProcessor) Decode(io.Reader) (image.Image, string, error)     { return nil, "jpeg", nil }
func (nilThumbnailProcessor) CreateThumbnail(image.Image, int, int) image.Image { return nil }
func (nilThumbnailProcessor) Encode(io.Writer, image.Image, string, int) error  { return nil }
func (nilThumbnailProcessor) ValidateMagicBytes([]byte) (string, error)         { return "", nil }

func TestHandleAvatarDeleted(t *testing.T) {
	storage := &workerStorageStub{}
	worker := &Worker{storage: storage}
	payload := `{"avatar_id":"avatar-1","s3_keys":["original","thumb"]}`
	if err := worker.handleAvatarDeleted(context.Background(), &contracts.Message{Payload: []byte(payload)}); err != nil {
		t.Fatal(err)
	}
	if len(storage.deleted) != 2 {
		t.Fatalf("deleted keys = %v", storage.deleted)
	}
	if err := worker.handleAvatarDeleted(context.Background(), &contracts.Message{Payload: []byte("bad")}); err == nil {
		t.Fatal("invalid deletion event accepted")
	}

	storage.deleteErr = errors.New("delete")
	if err := worker.handleAvatarDeleted(context.Background(), &contracts.Message{Payload: []byte(payload)}); err == nil {
		t.Fatal("storage deletion error ignored")
	}

	worker.blobRepo = workerBlobRepoStub{}
	if err := worker.handleAvatarDeleted(context.Background(), &contracts.Message{Payload: []byte(payload)}); err != nil {
		t.Fatalf("shared blob deletion should be deferred: %v", err)
	}
}

func TestMimeTypeForFormat(t *testing.T) {
	if mimeTypeForFormat("jpeg") != "image/jpeg" || mimeTypeForFormat("png") != "image/png" {
		t.Fatal("unexpected mime type")
	}
}

type workerAvatarRepoStub struct {
	avatar    *domain.Avatar
	getErr    error
	updateErr error
	updates   int
}

func (s *workerAvatarRepoStub) Create(context.Context, *domain.Avatar) error { return nil }
func (s *workerAvatarRepoStub) GetByID(context.Context, uuid.UUID) (*domain.Avatar, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.avatar, nil
}
func (s *workerAvatarRepoStub) GetByUserID(context.Context, string) (*domain.Avatar, error) {
	return s.avatar, nil
}
func (s *workerAvatarRepoStub) ListByUserID(context.Context, string, int, int) ([]*domain.Avatar, error) {
	return []*domain.Avatar{s.avatar}, nil
}
func (s *workerAvatarRepoStub) Update(_ context.Context, avatar *domain.Avatar) error {
	s.updates++
	if s.updateErr != nil {
		return s.updateErr
	}
	s.avatar = avatar
	return nil
}
func (s *workerAvatarRepoStub) Delete(context.Context, uuid.UUID) error { return nil }
func (s *workerAvatarRepoStub) GetPendingForProcessing(context.Context, int) ([]*domain.Avatar, error) {
	return nil, nil
}
func (s *workerAvatarRepoStub) UpdateProcessingStatus(context.Context, uuid.UUID, domain.ProcessingStatus) error {
	return nil
}

func avatarUploadedMessage(t *testing.T, avatarID uuid.UUID) *contracts.Message {
	t.Helper()
	payload, err := json.Marshal(events.AvatarUploadedEvent{
		AvatarID:   avatarID.String(),
		Operations: []events.ProcessingOperation{events.ProcessingOperationThumbnail100},
	})
	if err != nil {
		t.Fatal(err)
	}
	return &contracts.Message{Payload: payload}
}

func TestHandleAvatarUploadedValidationAndNotProcessable(t *testing.T) {
	worker := &Worker{avatarRepo: &workerAvatarRepoStub{getErr: domain.ErrNotFound}}
	if err := worker.handleAvatarUploaded(context.Background(), &contracts.Message{Payload: []byte("bad")}); err == nil {
		t.Fatal("invalid upload event accepted")
	}
	if err := worker.handleAvatarUploaded(context.Background(), &contracts.Message{Payload: []byte(`{"avatar_id":"bad"}`)}); err == nil {
		t.Fatal("invalid avatar id accepted")
	}
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, uuid.New())); err != nil {
		t.Fatalf("not found avatar should be ignored: %v", err)
	}

	avatar := &domain.Avatar{ID: uuid.New(), UploadStatus: domain.UploadStatusUploading, ProcessingStatus: domain.ProcessingStatusPending}
	worker.avatarRepo = &workerAvatarRepoStub{avatar: avatar}
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, avatar.ID)); err != nil {
		t.Fatalf("not processable avatar should be ignored: %v", err)
	}

	worker.avatarRepo = &workerAvatarRepoStub{getErr: errors.New("repository")}
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, uuid.New())); err == nil || !strings.Contains(err.Error(), "get avatar") {
		t.Fatalf("repository error = %v", err)
	}
}

func TestHandleAvatarUploadedSuccessAndProcessingErrors(t *testing.T) {
	newWorker := func(processor workerProcessorStub, storage *workerStorageStub, repo *workerAvatarRepoStub) *Worker {
		return &Worker{
			avatarRepo:    repo,
			thumbnailRepo: &workerThumbnailRepoStub{},
			storage:       storage,
			processor:     processor,
		}
	}
	newAvatar := func() *domain.Avatar {
		return &domain.Avatar{ID: uuid.New(), UserID: "user-1", UploadStatus: domain.UploadStatusCompleted, ProcessingStatus: domain.ProcessingStatusPending, S3KeyOriginal: "original.jpg"}
	}

	t.Run("success", func(t *testing.T) {
		avatar := newAvatar()
		repo := &workerAvatarRepoStub{avatar: avatar}
		worker := newWorker(workerProcessorStub{decodeImage: image.NewRGBA(image.Rect(0, 0, 20, 20)), decodeFormat: "jpeg"}, &workerStorageStub{}, repo)
		if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, avatar.ID)); err != nil {
			t.Fatal(err)
		}
		if avatar.ProcessingStatus != domain.ProcessingStatusCompleted || repo.updates < 2 {
			t.Fatalf("avatar was not processed: %+v updates=%d", avatar, repo.updates)
		}
	})

	tests := []struct {
		name      string
		processor workerProcessorStub
		storage   *workerStorageStub
		repo      *workerAvatarRepoStub
		want      string
	}{
		{name: "download error", processor: workerProcessorStub{}, storage: &workerStorageStub{downloadErr: errors.New("download")}, repo: &workerAvatarRepoStub{avatar: newAvatar()}, want: "download original"},
		{name: "decode error", processor: workerProcessorStub{decodeErr: errors.New("decode")}, storage: &workerStorageStub{}, repo: &workerAvatarRepoStub{avatar: newAvatar()}, want: "decode original"},
		{name: "mark processing error", processor: workerProcessorStub{}, storage: &workerStorageStub{}, repo: &workerAvatarRepoStub{avatar: newAvatar(), updateErr: errors.New("update")}, want: "mark avatar processing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := newWorker(tt.processor, tt.storage, tt.repo)
			err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, tt.repo.avatar.ID))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

type workerBlobRepoRich struct {
	blob       *domain.Blob
	derivation *domain.BlobDerivation
	linked     bool
}

func (s *workerBlobRepoRich) GetOrCreate(context.Context, *domain.Blob) (*domain.Blob, error) {
	return s.blob, nil
}
func (*workerBlobRepoRich) MarkReady(context.Context, uuid.UUID) error          { return nil }
func (*workerBlobRepoRich) MarkFailed(context.Context, uuid.UUID, string) error { return nil }
func (s *workerBlobRepoRich) EnsureDerivation(_ context.Context, derivation *domain.BlobDerivation) (*domain.BlobDerivation, error) {
	if s.derivation != nil {
		return s.derivation, nil
	}
	s.derivation = derivation
	return derivation, nil
}
func (s *workerBlobRepoRich) LinkThumbnail(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	s.linked = true
	return nil
}

type workerOutboxStub struct {
	events       []*contracts.OutboxEvent
	claimErr     error
	markPubErr   error
	markedFailed bool
	markedPub    bool
}

func (s *workerOutboxStub) Save(context.Context, *contracts.OutboxEvent) error { return nil }
func (s *workerOutboxStub) ClaimPending(context.Context, int, time.Duration) ([]*contracts.OutboxEvent, error) {
	return s.events, s.claimErr
}
func (s *workerOutboxStub) ListPending(context.Context, int) ([]*contracts.OutboxEvent, error) {
	return s.events, nil
}
func (s *workerOutboxStub) MarkPublished(context.Context, string) error {
	s.markedPub = true
	return s.markPubErr
}
func (s *workerOutboxStub) MarkFailed(context.Context, string, string, time.Time) error {
	s.markedFailed = true
	return nil
}

type workerPublisherStub struct {
	err error
}

func (s workerPublisherStub) Publish(context.Context, string, *contracts.Message) error { return s.err }

func TestCreateThumbnailWithSharedBlob(t *testing.T) {
	blob := &domain.Blob{ID: uuid.New(), ObjectKey: "blobs/derived", StorageStatus: domain.BlobStorageStatusReady}
	blobRepo := &workerBlobRepoRich{blob: blob}
	storage := &workerStorageStub{}
	thumbnailRepo := &workerThumbnailRepoStub{}
	worker := &Worker{
		storage:       storage,
		processor:     workerProcessorStub{},
		thumbnailRepo: thumbnailRepo,
		blobRepo:      blobRepo,
	}
	avatar := &domain.Avatar{ID: uuid.New(), UserID: "user-1", SourceBlobID: uuid.New()}
	if err := worker.createThumbnail(context.Background(), avatar, image.NewRGBA(image.Rect(0, 0, 20, 20)), domain.ThumbnailSize100x100, "webp"); err != nil {
		t.Fatal(err)
	}
	if len(storage.uploaded) != 0 || thumbnailRepo.created == nil || thumbnailRepo.created.BlobID != blob.ID || !blobRepo.linked {
		t.Fatalf("shared blob path not used: uploads=%v record=%+v linked=%v", storage.uploaded, thumbnailRepo.created, blobRepo.linked)
	}
}

func TestFlushOutboxAndRetryDelay(t *testing.T) {
	event := &contracts.OutboxEvent{ID: "event-1", Topic: "avatar.uploaded", Payload: []byte("{}"), Attempts: 1}
	outbox := &workerOutboxStub{events: []*contracts.OutboxEvent{event}}
	worker := &Worker{outbox: outbox, publisher: workerPublisherStub{}}
	worker.flushOutbox(context.Background())
	if !outbox.markedPub || outbox.markedFailed {
		t.Fatalf("successful outbox event state: published=%v failed=%v", outbox.markedPub, outbox.markedFailed)
	}

	failedOutbox := &workerOutboxStub{events: []*contracts.OutboxEvent{event}}
	failedWorker := &Worker{outbox: failedOutbox, publisher: workerPublisherStub{err: errors.New("publish")}}
	failedWorker.flushOutbox(context.Background())
	if !failedOutbox.markedFailed {
		t.Fatal("failed publish was not marked failed")
	}

	if got := outboxRetryDelay(0); got != time.Second {
		t.Fatalf("retry delay for zero attempt = %s", got)
	}
	if got := outboxRetryDelay(10); got != 256*time.Second {
		t.Fatalf("retry delay cap = %s", got)
	}
	if got := outboxRetryDelay(3); got != 4*time.Second {
		t.Fatalf("retry delay = %s", got)
	}
}

func TestFailAvatarAndInvalidDeliveryMetadata(t *testing.T) {
	avatar := &domain.Avatar{UploadStatus: domain.UploadStatusCompleted, ProcessingStatus: domain.ProcessingStatusProcessing}
	repo := &workerAvatarRepoStub{avatar: avatar}
	worker := &Worker{avatarRepo: repo}
	if err := worker.failAvatar(context.Background(), avatar, errors.New("failure"), false); err == nil || avatar.ProcessingStatus != domain.ProcessingStatusPending {
		t.Fatalf("retry failure state: err=%v avatar=%+v", err, avatar)
	}
	if err := worker.failAvatar(context.Background(), avatar, errors.New("final"), true); err == nil || avatar.ProcessingStatus != domain.ProcessingStatusFailed {
		t.Fatalf("final failure state: err=%v avatar=%+v", err, avatar)
	}
	for _, headers := range []map[string]string{
		{contracts.DeliveryAttemptHeader: "bad", contracts.MaxDeliveryAttemptsHeader: "5"},
		{contracts.DeliveryAttemptHeader: "0", contracts.MaxDeliveryAttemptsHeader: "5"},
		{contracts.DeliveryAttemptHeader: "1", contracts.MaxDeliveryAttemptsHeader: "0"},
	} {
		if !isFinalDelivery(&contracts.Message{Headers: headers}) {
			t.Errorf("invalid headers treated as retryable: %v", headers)
		}
	}
}

type workerConsumerStub struct {
	err    error
	calls  int
	topics []string
}

func (s *workerConsumerStub) Subscribe(_ context.Context, topic string, _ contracts.MessageHandler) error {
	s.calls++
	s.topics = append(s.topics, topic)
	return s.err
}

func TestWorkerStartSubscribesAndStopsWithContext(t *testing.T) {
	consumer := &workerConsumerStub{}
	worker := &Worker{broker: consumer}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := worker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if consumer.calls != 2 || len(consumer.topics) != 2 {
		t.Fatalf("subscriptions = %v calls=%d", consumer.topics, consumer.calls)
	}
}

func TestWorkerStartReturnsSubscribeError(t *testing.T) {
	worker := &Worker{broker: &workerConsumerStub{err: errors.New("subscribe")}}
	if err := worker.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "subscribe to avatar.uploaded") {
		t.Fatalf("subscribe error = %v", err)
	}
}

func TestDispatchOutboxStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := &Worker{outbox: &workerOutboxStub{}, publisher: workerPublisherStub{}}
	worker.dispatchOutbox(ctx)
}

type workerClaimerRepoStub struct {
	workerAvatarRepoStub
	claimed  *domain.Avatar
	claimErr error
}

func (s *workerClaimerRepoStub) ClaimForProcessing(context.Context, uuid.UUID) (*domain.Avatar, error) {
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return s.claimed, nil
}

func TestHandleAvatarUploadedClaimerBranches(t *testing.T) {
	newAvatar := func() *domain.Avatar {
		return &domain.Avatar{ID: uuid.New(), UserID: "user-1", UploadStatus: domain.UploadStatusCompleted, ProcessingStatus: domain.ProcessingStatusPending, S3KeyOriginal: "original.jpg"}
	}
	avatar := newAvatar()
	claimed := newAvatar()
	claimed.ID = avatar.ID
	worker := &Worker{
		avatarRepo:    &workerClaimerRepoStub{workerAvatarRepoStub: workerAvatarRepoStub{avatar: avatar}, claimed: claimed},
		thumbnailRepo: &workerThumbnailRepoStub{},
		storage:       &workerStorageStub{},
		processor:     workerProcessorStub{decodeImage: image.NewRGBA(image.Rect(0, 0, 20, 20)), decodeFormat: "jpeg"},
	}
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, avatar.ID)); err != nil {
		t.Fatalf("claimer success: %v", err)
	}

	notFound := &workerClaimerRepoStub{workerAvatarRepoStub: workerAvatarRepoStub{avatar: avatar}, claimErr: domain.ErrNotFound}
	worker.avatarRepo = notFound
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, avatar.ID)); err != nil {
		t.Fatalf("already claimed avatar should be ignored: %v", err)
	}

	claimError := &workerClaimerRepoStub{workerAvatarRepoStub: workerAvatarRepoStub{avatar: avatar}, claimErr: errors.New("claim")}
	worker.avatarRepo = claimError
	if err := worker.handleAvatarUploaded(context.Background(), avatarUploadedMessage(t, avatar.ID)); err == nil || !strings.Contains(err.Error(), "claim avatar for processing") {
		t.Fatalf("claim error = %v", err)
	}
}
