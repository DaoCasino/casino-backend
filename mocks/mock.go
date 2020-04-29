package mocks

import (
    "context"
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
