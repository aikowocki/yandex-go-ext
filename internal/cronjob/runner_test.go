package cronjob

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aikowocki/yandex-go-ext/internal/config"
	"github.com/aikowocki/yandex-go-ext/internal/contracts"
	"github.com/aikowocki/yandex-go-ext/internal/domain"
	"github.com/aikowocki/yandex-go-ext/internal/shared/logging"
	"github.com/aikowocki/yandex-go-ext/internal/shared/testsupport"
	"github.com/google/uuid"
)

type passthroughTx struct{}

func (passthroughTx) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type retentionAvatarStub struct{ calls int }

func (s *retentionAvatarStub) HardDeleteExpired(context.Context, time.Time) (int64, error) {
	s.calls++
	return 2, nil
}

type retentionThumbnailStub struct{ calls int }

func (s *retentionThumbnailStub) HardDeleteExpired(context.Context, time.Time) (int64, error) {
	s.calls++
	return 3, nil
}

type retentionBlobStub struct {
	candidate *domain.Blob
	deleted   []uuid.UUID
	known     map[string]bool
	events    *[]string
}

func (s *retentionBlobStub) ListUnreferenced(context.Context, time.Time, int) ([]*domain.Blob, error) {
	if s.candidate == nil {
		return nil, nil
	}
	return []*domain.Blob{s.candidate}, nil
}
func (s *retentionBlobStub) ClaimForDeletion(_ context.Context, id uuid.UUID) (*domain.Blob, bool, error) {
	if s.candidate == nil || s.candidate.ID != id {
		return nil, false, nil
	}
	return s.candidate, true, nil
}
func (s *retentionBlobStub) Delete(_ context.Context, id uuid.UUID) error {
	s.deleted = append(s.deleted, id)
	if s.events != nil {
		*s.events = append(*s.events, "blob")
	}
	return nil
}
func (s *retentionBlobStub) HasObject(_ context.Context, key string) (bool, error) {
	return s.known[key], nil
}

type retentionStorageStub struct {
	deleted []string
	objects []contracts.StoredObject
	events  *[]string
}

func (s *retentionStorageStub) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}
func (s *retentionStorageStub) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}
func (s *retentionStorageStub) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	if s.events != nil {
		*s.events = append(*s.events, "storage")
	}
	return nil
}
func (s *retentionStorageStub) GetURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s *retentionStorageStub) List(_ context.Context, _ string, _ time.Time) ([]contracts.StoredObject, error) {
	return s.objects, nil
}

func TestRunRetentionDeletesExpiredDataAndUnreferencedBlob(t *testing.T) {
	avatarRetention := &retentionAvatarStub{}
	thumbnailRetention := &retentionThumbnailStub{}
	blobID := uuid.New()
	events := []string{}
	blobRetention := &retentionBlobStub{
		candidate: &domain.Blob{ID: blobID, ObjectKey: "blobs/orphan"},
		events:    &events,
	}
	storage := &retentionStorageStub{events: &events}
	runner := New(avatarRetention, thumbnailRetention, blobRetention, storage, nil, passthroughTx{}, config.WorkerConfig{}, logging.ComponentLogger(nil, "cronjob"))
	ctx := testsupport.Context(t)

	if err := runner.RunRetention(ctx); err != nil {
		t.Fatal(err)
	}
	if avatarRetention.calls != 1 || thumbnailRetention.calls != 1 {
		t.Fatalf("unexpected retention calls: avatars=%d thumbnails=%d", avatarRetention.calls, thumbnailRetention.calls)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != "blobs/orphan" {
		t.Fatalf("unexpected storage deletions: %v", storage.deleted)
	}
	if len(blobRetention.deleted) != 1 || blobRetention.deleted[0] != blobID {
		t.Fatalf("unexpected blob deletions: %v", blobRetention.deleted)
	}
	if len(events) != 2 || events[0] != "storage" || events[1] != "blob" {
		t.Fatalf("unexpected deletion order: %v", events)
	}
	testsupport.Assert(t).
		Contains("cronjob hard-deleted expired avatars").
		HasField("component", "cronjob")
}

func TestRunReconcileDeletesOnlyUntrackedObjects(t *testing.T) {
	blobRetention := &retentionBlobStub{known: map[string]bool{"blobs/linked": true}}
	storage := &retentionStorageStub{objects: []contracts.StoredObject{
		{Key: "blobs/linked"},
		{Key: "blobs/orphan"},
	}}
	runner := New(nil, nil, blobRetention, storage, storage, passthroughTx{}, config.WorkerConfig{}, logging.ComponentLogger(nil, "cronjob"))
	ctx := testsupport.Context(t)

	if err := runner.RunReconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != "blobs/orphan" {
		t.Fatalf("unexpected storage deletions: %v", storage.deleted)
	}
	testsupport.Assert(t).
		Contains("cronjob deleted storage orphan").
		HasField("component", "cronjob")
}

type cronErrorBlobStub struct{ blob *domain.Blob }

func (cronErrorBlobStub) ListUnreferenced(context.Context, time.Time, int) ([]*domain.Blob, error) {
	return nil, errors.New("list")
}
func (s cronErrorBlobStub) ClaimForDeletion(context.Context, uuid.UUID) (*domain.Blob, bool, error) {
	return s.blob, true, errors.New("claim")
}
func (cronErrorBlobStub) Delete(context.Context, uuid.UUID) error { return errors.New("delete row") }
func (cronErrorBlobStub) HasObject(context.Context, string) (bool, error) {
	return false, errors.New("check")
}

type cronErrorRetentionStub struct{}

func (cronErrorRetentionStub) HardDeleteExpired(context.Context, time.Time) (int64, error) {
	return 0, errors.New("retention")
}

type cronErrorStorageStub struct{ listErr, deleteErr error }

func (cronErrorStorageStub) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}
func (cronErrorStorageStub) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}
func (s cronErrorStorageStub) Delete(context.Context, string) error { return s.deleteErr }
func (cronErrorStorageStub) GetURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s cronErrorStorageStub) List(context.Context, string, time.Time) ([]contracts.StoredObject, error) {
	return nil, s.listErr
}

func TestRunnerGuardsAndErrorPaths(t *testing.T) {
	var nilRunner *Runner
	if err := nilRunner.RunRetention(context.Background()); err == nil {
		t.Fatal("nil retention runner accepted")
	}
	if err := nilRunner.RunReconcile(context.Background()); err == nil {
		t.Fatal("nil reconcile runner accepted")
	}
	runner := New(cronErrorRetentionStub{}, cronErrorRetentionStub{}, cronErrorBlobStub{}, cronErrorStorageStub{listErr: errors.New("list")}, nil, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunRetention(context.Background()); err == nil {
		t.Fatal("retention errors were ignored")
	}
	if err := runner.RunReconcile(context.Background()); err == nil {
		t.Fatal("missing storage lister was accepted")
	}
	storage := cronErrorStorageStub{listErr: errors.New("list")}
	runner = New(nil, nil, cronErrorBlobStub{}, storage, storage, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunReconcile(context.Background()); err == nil {
		t.Fatal("reconcile list error was ignored")
	}
}

type cronBranchBlobStub struct {
	candidates []*domain.Blob
	claimBlob  *domain.Blob
	claimed    bool
	claimErr   error
	deleteErr  error
	known      bool
	hasErr     error
}

func (s cronBranchBlobStub) ListUnreferenced(context.Context, time.Time, int) ([]*domain.Blob, error) {
	return s.candidates, nil
}
func (s cronBranchBlobStub) ClaimForDeletion(context.Context, uuid.UUID) (*domain.Blob, bool, error) {
	return s.claimBlob, s.claimed, s.claimErr
}
func (s cronBranchBlobStub) Delete(context.Context, uuid.UUID) error { return s.deleteErr }
func (s cronBranchBlobStub) HasObject(context.Context, string) (bool, error) {
	return s.known, s.hasErr
}

type cronBranchStorageStub struct {
	objects   []contracts.StoredObject
	deleteErr error
}

func (s cronBranchStorageStub) Upload(context.Context, string, io.Reader, int64, string) error {
	return nil
}
func (s cronBranchStorageStub) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}
func (s cronBranchStorageStub) Delete(context.Context, string) error { return s.deleteErr }
func (s cronBranchStorageStub) GetURL(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s cronBranchStorageStub) List(context.Context, string, time.Time) ([]contracts.StoredObject, error) {
	return s.objects, nil
}

func TestRunnerRetentionAndReconcileCandidateBranches(t *testing.T) {
	blob := &domain.Blob{ID: uuid.New(), ObjectKey: "blobs/orphan"}
	candidates := []*domain.Blob{nil, {ID: uuid.New()}, {ID: uuid.New(), ObjectKey: ""}, blob}
	retention := cronBranchBlobStub{candidates: candidates, claimBlob: blob, claimed: false}
	runner := New(nil, nil, retention, cronBranchStorageStub{}, nil, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunRetention(context.Background()); err != nil {
		t.Fatal(err)
	}

	retention = cronBranchBlobStub{candidates: []*domain.Blob{blob}, claimBlob: blob, claimed: true, claimErr: errors.New("claim")}
	runner = New(nil, nil, retention, cronBranchStorageStub{}, nil, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunRetention(context.Background()); err == nil {
		t.Fatal("claim error ignored")
	}

	retention = cronBranchBlobStub{candidates: []*domain.Blob{blob}, claimBlob: blob, claimed: true, deleteErr: errors.New("delete row")}
	runner = New(nil, nil, retention, cronBranchStorageStub{}, nil, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunRetention(context.Background()); err == nil {
		t.Fatal("blob row delete error ignored")
	}

	objects := []contracts.StoredObject{{Key: "linked"}, {Key: "orphan"}}
	retention = cronBranchBlobStub{known: true, hasErr: errors.New("check")}
	storage := cronBranchStorageStub{objects: objects, deleteErr: errors.New("delete object")}
	runner = New(nil, nil, retention, storage, storage, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunReconcile(context.Background()); err == nil {
		t.Fatal("reconcile object errors ignored")
	}
}

func TestRunnerRetentionAndReconcileSuccessPaths(t *testing.T) {
	ctx := testsupport.Context(t)
	blob := &domain.Blob{ID: uuid.New(), ObjectKey: "blobs/orphan"}
	retention := cronBranchBlobStub{
		candidates: []*domain.Blob{blob},
		claimBlob:  blob,
		claimed:    true,
	}
	storage := cronBranchStorageStub{}
	runner := New(nil, nil, retention, storage, storage, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunRetention(ctx); err != nil {
		t.Fatalf("retention success path: %v", err)
	}

	retention = cronBranchBlobStub{}
	storage = cronBranchStorageStub{objects: []contracts.StoredObject{{Key: "blobs/orphan"}}}
	runner = New(nil, nil, retention, storage, storage, passthroughTx{}, config.WorkerConfig{}, nil)
	if err := runner.RunReconcile(ctx); err != nil {
		t.Fatalf("reconcile success path: %v", err)
	}

	testsupport.Assert(t).
		Contains("cronjob deleted orphan blob").
		Contains("cronjob deleted storage orphan").
		NoErrors()
}
