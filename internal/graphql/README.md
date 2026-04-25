# GraphQL Interface for HubProxy

This package implements a GraphQL interface for HubProxy that mirrors the functionality of the REST API.

## Endpoints

The GraphQL interface is available at `/graphql` on the API server.

## Queries

### `events` - List webhook events

```graphql
query {
  events(
    type: String
    repository: String
    sender: String
    since: DateTime
    until: DateTime
    limit: Int
    offset: Int
  ) {
    events {
      id
      type
      payload
      createdAt
      error
      repository
      sender
      replayedFrom
      ReplayedTime
    }
    total
  }
}
```

### `event` - Get a single webhook event by ID

```graphql
query {
  event(id: "event-id") {
    id
    type
    payload
    createdAt
    error
    repository
    sender
    replayedFrom
    ReplayedTime
  }
}
```

### `stats` - Get webhook event statistics

```graphql
query {
  stats(since: "2023-01-01T00:00:00Z") {
    type
    count
  }
}
```

## Mutations

### `replayEvent` - Replay a single webhook event

This mutation updates the existing stored event in place. The returned `id` remains the original delivery ID, while `replayedFrom` and `ReplayedTime` describe the latest replay request.

```graphql
mutation {
  replayEvent(id: "event-id") {
    replayedCount
    events {
      id
      type
      payload
      createdAt
      repository
      sender
      replayedFrom
      ReplayedTime
    }
  }
}
```

### `replayRange` - Replay multiple webhook events within a time range

This mutation updates each matching stored event in place rather than inserting separate replay rows. Replaying the same event again overwrites the previous replay metadata.

```graphql
mutation {
  replayRange(
    since: "2023-01-01T00:00:00Z"
    until: "2023-01-02T00:00:00Z"
    type: String
    repository: String
    sender: String
    limit: Int
  ) {
    replayedCount
    events {
      id
      type
      payload
      createdAt
      repository
      sender
      replayedFrom
      ReplayedTime
    }
  }
}
```

## Interactive Tools

The GraphQL endpoint includes:

- **GraphiQL**: An interactive in-browser GraphQL IDE
- **Playground**: An alternative GraphQL IDE

These tools are available directly at the `/graphql` endpoint when accessed from a browser.
