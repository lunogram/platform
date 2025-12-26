# PubSub Event Processing

This package handles asynchronous event processing using NATS JetStream for the Lunogram platform.

## Architecture Overview

Events flow through a multi-stage pipeline that handles storage, schema extraction, dependency resolution, and triggers downstream processing for lists and journeys.

## Event Flow

```mermaid
graph TB
    Client[Client/API] -->|Submit Event| EventSubject["events.projects.{project_id}"]
    Client -->|Submit User| UserSubject[["users.projects.{project_id}"]]

    EventSubject -->|Consume| EventHandler["Event Handler"]
    UserSubject -->|Consume| UserHandler["User Handler"]

    %% Schema publishing
    EventHandler --> EventSchemaSubject[["events.schemas.{project_id}"]]
    UserHandler --> UserSchemaSubject[["users.schemas.{project_id}"]]

    %% Recompute triggers from events
    EventHandler -->|Affected Lists| ListSubject[["recompute.lists.{project_id}.{list_id}"]]
    EventHandler -->|Affected Journeys| JourneySubject[["recompute.journeys.{project_id}.{journey_id}"]]

    %% Recompute triggers from users
    UserHandler -->|Affected Lists| ListSubject
    UserHandler -->|User system events| EventSubject

    %% Schema handlers
    EventSchemaSubject -->|Consume| EventSchemaHandler["Event Schema Handler"]
    UserSchemaSubject -->|Consume| UserSchemaHandler["User Schema Handler"]

    ListSubject -->|Consume| ListRecompute["List Recomputation"]
    JourneySubject -->|Consume| JourneyRecompute["Journey Recomputation"]

    ListRecompute -->|Publish Jobs| ListJobSubject[["jobs.lists.{project_id}.{list_id}"]]
    ListRecompute -->|List Membership Change| JourneyJobSubject[["jobs.journeys.{project_id}.{journey_id}"]]

    JourneyRecompute -->|Publish Jobs| JourneyJobSubject

    ListJobSubject -->|Consume| ListJobHandler["List Job Handler"]
    JourneyJobSubject -->|Consume| JourneyJobHandler["Journey Job Handler"]
```
