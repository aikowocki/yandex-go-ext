package app

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewPprofServerEmptyAddressDisabled(t *testing.T) {
	assert.Nil(t, newPprofServer(""))
}

func TestNewPprofServerServesIndexWhenEnabled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	server := newPprofServer(address)
	require.NotNil(t, server)
	assert.Equal(t, address, server.Addr)

	go func() { _ = server.ListenAndServe() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	var response *http.Response
	require.Eventually(t, func() bool {
		var requestErr error
		response, requestErr = http.Get("http://" + address + "/debug/pprof/")
		return requestErr == nil
	}, 2*time.Second, 20*time.Millisecond)
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "pprof")
}
func TestContainerRunRejectsUninitializedContainer(t *testing.T) {
	var nilContainer *Container
	if err := nilContainer.Run(); err == nil {
		t.Fatal("nil container accepted")
	}
	if err := (&Container{}).Run(); err == nil {
		t.Fatal("uninitialized container accepted")
	}
}

func TestContainerCloseIsNilSafeAndShutdownCanCloseEmptyContainer(t *testing.T) {
	var nilContainer *Container
	if err := nilContainer.Close(); err != nil {
		t.Fatal(err)
	}
	container := &Container{}
	if err := container.Close(); err != nil {
		t.Fatal(err)
	}
	container.shutdown(context.Background(), nil)
}
