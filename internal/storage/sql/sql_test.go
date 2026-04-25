package sql_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hubproxy/internal/storage"
	"hubproxy/internal/storage/sql"
)

func TestDuplicateEventHandling(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	// Create initial event
	event := &storage.Event{
		ID:         "test-event-1",
		Type:       "push",
		Payload:    []byte(`{"ref": "refs/heads/main"}`),
		Headers:    []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["test-event-1"]}`),
		CreatedAt:  time.Now().UTC(),
		Repository: "test/repo",
		Sender:     "test-user",
	}

	// Store the event first time
	err = store.StoreEvent(ctx, event)
	require.NoError(t, err)

	// Verify event was stored
	stored, err := store.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)

	// Try to store the same event with different status

	err = store.StoreEvent(ctx, event)
	require.NoError(t, err)

	// Verify the original event was not modified
	stored, err = store.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)

	// Count events to ensure no duplicates
	count, err := store.CountEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have exactly one event")
}

func TestConcurrentEventInsertion(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test_concurrent.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	// Create base event
	event := &storage.Event{
		ID:         "concurrent-test-1",
		Type:       "push",
		Payload:    []byte(`{"ref": "refs/heads/main"}`),
		Headers:    []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["concurrent-test-1"]}`),
		CreatedAt:  time.Now().UTC(),
		Repository: "test/repo",
		Sender:     "test-user",
	}

	// Store the initial event
	err = store.StoreEvent(ctx, event)
	require.NoError(t, err)

	// Simulate concurrent insertions with the same ID
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			e := *event // Create a copy
			storeErr := store.StoreEvent(ctx, &e)
			require.NoError(t, storeErr)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify only one event exists
	count, err := store.CountEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Should have exactly one event despite concurrent insertions")

	// Verify the first event's status is unchanged (since duplicates are ignored)
	stored, err := store.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
}

func TestEventHeadersHandling(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test_headers.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	// Create events with different headers
	events := []*storage.Event{
		{
			ID:         "test-headers-1",
			Type:       "push",
			Payload:    []byte(`{"ref": "refs/heads/main"}`),
			Headers:    []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["test-headers-1"]}`),
			CreatedAt:  time.Now().UTC(),
			Repository: "test/repo",
			Sender:     "test-user",
		},
		{
			ID:         "test-headers-2",
			Type:       "pull_request",
			Payload:    []byte(`{"action": "opened"}`),
			Headers:    []byte(`{"X-GitHub-Event": ["pull_request"], "X-GitHub-Delivery": ["test-headers-2"], "X-Custom-Header": ["test-value"]}`),
			CreatedAt:  time.Now().UTC(),
			Repository: "test/repo",
			Sender:     "test-user",
		},
	}

	// Store both events
	for _, event := range events {
		err = store.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// Test GetEvent headers retrieval
	var stored *storage.Event
	for _, expected := range events {
		stored, err = store.GetEvent(ctx, expected.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)
		assert.Equal(t, expected.Headers, stored.Headers, "Headers should match for event %s", expected.ID)
	}

	// Test ListEvents headers retrieval
	listed, total, err := store.ListEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, total, "Should have exactly two events")
	assert.Len(t, listed, 2, "Should list both events")

	// Create a map of expected events by ID for easier comparison
	expectedByID := make(map[string]*storage.Event)
	for _, e := range events {
		expectedByID[e.ID] = e
	}

	// Verify headers in listed events
	for _, actual := range listed {
		expected := expectedByID[actual.ID]
		require.NotNil(t, expected, "Should find matching expected event")
		assert.Equal(t, expected.Headers, actual.Headers, "Headers should match for event %s", actual.ID)
	}
}

func TestForwardedAtField(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test_forwarded.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	// Create current time for testing
	now := time.Now().UTC()
	forwardedTime := now.Add(5 * time.Minute)

	// Create events with different forwarded_at values
	events := []*storage.Event{
		{
			ID:          "test-forwarded-1",
			Type:        "push",
			Payload:     []byte(`{"ref": "refs/heads/main"}`),
			Headers:     []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["test-forwarded-1"]}`),
			CreatedAt:   now,
			ForwardedAt: &forwardedTime, // Event has been forwarded
			Repository:  "test/repo",
			Sender:      "test-user",
		},
		{
			ID:          "test-forwarded-2",
			Type:        "pull_request",
			Payload:     []byte(`{"action": "opened"}`),
			Headers:     []byte(`{"X-GitHub-Event": ["pull_request"], "X-GitHub-Delivery": ["test-forwarded-2"]}`),
			CreatedAt:   now,
			ForwardedAt: nil, // Event has not been forwarded
			Repository:  "test/repo",
			Sender:      "test-user",
		},
	}

	// Store both events
	for _, event := range events {
		err = store.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// Test GetEvent forwarded_at retrieval
	var stored *storage.Event
	for _, expected := range events {
		stored, err = store.GetEvent(ctx, expected.ID)
		require.NoError(t, err)
		require.NotNil(t, stored)

		if expected.ForwardedAt == nil {
			assert.Nil(t, stored.ForwardedAt, "ForwardedAt should be nil for event %s", expected.ID)
		} else {
			assert.NotNil(t, stored.ForwardedAt, "ForwardedAt should not be nil for event %s", expected.ID)
			assert.Equal(t, expected.ForwardedAt.Unix(), stored.ForwardedAt.Unix(), "ForwardedAt should match for event %s", expected.ID)
		}
	}

	// Test ListEvents forwarded_at retrieval
	listed, total, err := store.ListEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, total, "Should have exactly two events")
	assert.Len(t, listed, 2, "Should list both events")

	// Test filtering for non-forwarded events only
	nonForwardedEvents, nonForwardedTotal, err := store.ListEvents(ctx, storage.QueryOptions{OnlyNonForwarded: true})
	require.NoError(t, err)
	assert.Equal(t, 1, nonForwardedTotal, "Should have exactly one non-forwarded event")
	assert.Len(t, nonForwardedEvents, 1, "Should list only non-forwarded events")
	assert.Equal(t, "test-forwarded-2", nonForwardedEvents[0].ID, "Should be the non-forwarded event")
	assert.Nil(t, nonForwardedEvents[0].ForwardedAt, "ForwardedAt should be nil for the non-forwarded event")

	// Create a map of expected events by ID for easier comparison
	expectedByID := make(map[string]*storage.Event)
	for _, e := range events {
		expectedByID[e.ID] = e
	}

	// Verify forwarded_at in listed events
	for _, actual := range listed {
		expected := expectedByID[actual.ID]
		require.NotNil(t, expected, "Should find matching expected event")

		if expected.ForwardedAt == nil {
			assert.Nil(t, actual.ForwardedAt, "ForwardedAt should be nil for event %s", expected.ID)
		} else {
			assert.NotNil(t, actual.ForwardedAt, "ForwardedAt should not be nil for event %s", expected.ID)
			assert.Equal(t, expected.ForwardedAt.Unix(), actual.ForwardedAt.Unix(), "ForwardedAt should match for event %s", expected.ID)
		}
	}
}

func TestUpdateEvent(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test_update.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	createdAt := time.Now().UTC().Truncate(time.Second)
	forwardedAt := createdAt.Add(2 * time.Minute)
	replayedAt := createdAt.Add(5 * time.Minute)

	event := &storage.Event{
		ID:         "test-update-1",
		Type:       "push",
		Payload:    []byte(`{"ref": "refs/heads/main"}`),
		Headers:    []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["test-update-1"]}`),
		CreatedAt:  createdAt,
		Repository: "test/repo",
		Sender:     "test-user",
	}

	err = store.StoreEvent(ctx, event)
	require.NoError(t, err)

	updated := &storage.Event{
		ID:           event.ID,
		Type:         "push",
		Payload:      []byte(`{"ref": "refs/heads/release"}`),
		Headers:      []byte(`{"X-GitHub-Event": ["push"], "X-GitHub-Delivery": ["test-update-1"], "X-Replayed": ["true"]}`),
		CreatedAt:    createdAt,
		ForwardedAt:  &forwardedAt,
		Error:        "replayed successfully",
		Repository:   "test/repo",
		Sender:       "test-user",
		ReplayedFrom: "original-event-1",
		ReplayedTime: replayedAt,
	}

	err = store.UpdateEvent(ctx, updated)
	require.NoError(t, err)

	stored, err := store.GetEvent(ctx, event.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, updated.Payload, stored.Payload)
	assert.Equal(t, updated.Headers, stored.Headers)
	assert.NotNil(t, stored.ForwardedAt)
	assert.Equal(t, updated.ForwardedAt.Unix(), stored.ForwardedAt.Unix())
	assert.Equal(t, updated.Error, stored.Error)
	assert.Equal(t, updated.ReplayedFrom, stored.ReplayedFrom)
	assert.True(t, stored.ReplayedTime.Equal(updated.ReplayedTime))

	listed, total, err := store.ListEvents(ctx, storage.QueryOptions{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, listed, 1)
	assert.Equal(t, updated.ReplayedFrom, listed[0].ReplayedFrom)
	assert.True(t, listed[0].ReplayedTime.Equal(updated.ReplayedTime))

	count, err := store.CountEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestUpdateEventMissingRow(t *testing.T) {
	ctx := context.Background()
	store, err := sql.New("sqlite:file:test_update_missing.db?mode=memory&cache=shared")
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	err = store.UpdateEvent(ctx, &storage.Event{
		ID:           "missing-event",
		Type:         "push",
		Payload:      []byte(`{"ref": "refs/heads/main"}`),
		Headers:      []byte(`{"X-GitHub-Event": ["push"]}`),
		CreatedAt:    now,
		Repository:   "test/repo",
		Sender:       "test-user",
		ReplayedFrom: "original-event-1",
		ReplayedTime: now.Add(time.Minute),
	})
	require.NoError(t, err)

	stored, err := store.GetEvent(ctx, "missing-event")
	require.NoError(t, err)
	assert.Nil(t, stored)

	count, err := store.CountEvents(ctx, storage.QueryOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
