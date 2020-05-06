package mocks

import (
	"bytes"
	"context"
	"sync"

	broker "github.com/DaoCasino/platform-action-monitor-client"
)

type EventListenerMock struct{}

func (e *EventListenerMock) ListenAndServe(ctx context.Context) error {
	return nil
}

func (e *EventListenerMock) Subscribe(eventType broker.EventType, offset uint64) (bool, error) {
	return true, nil
}

func (e *EventListenerMock) Unsubscribe(eventType broker.EventType) (bool, error) {
	return true, nil
}

func (e *EventListenerMock) Run(ctx context.Context) {
}

type SafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *SafeBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *SafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *SafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func (b *SafeBuffer) Reset() {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Reset()
}
