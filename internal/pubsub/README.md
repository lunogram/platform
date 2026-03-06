# PubSub Event Processing

This package handles asynchronous event processing using NATS JetStream for the Lunogram platform.

## Architecture Overview

Events flow through a multi-stage pipeline that handles storage, schema extraction, dependency resolution, and triggers downstream processing for lists and journeys.

## Event Flow

```mermaid
graph TB
    Client[Client/API] -->|Submit User Event| EventSubject["users.events.process.{project_id}"]
    Client -->|Submit User| UserSubject["users.process.{project_id}"]
    Client -->|Submit Organization| OrgSubject["organizations.process.{project_id}"]
    Client -->|Submit Org User| OrgUserSubject["organizations.users.process.{project_id}"]
    Client -->|Submit Org Event| OrgEventSubject["organizations.events.process.{project_id}"]
    Client -->|Send Campaign| CampaignSubject["campaigns.send.{project_id}.{campaign_id}"]
    Client -->|Execute Action| ActionSubject["actions.execute.{project_id}"]

    EventSubject -->|Consume| EventHandler["User Event Handler"]
    UserSubject -->|Consume| UserHandler["User Handler"]
    OrgSubject -->|Consume| OrgHandler["Organization Handler"]
    OrgUserSubject -->|Consume| OrgUserHandler["Organization User Handler"]
    OrgEventSubject -->|Consume| OrgEventHandler["Organization Event Handler"]
    CampaignSubject -->|Consume| CampaignHandler["Campaign Send Handler"]
    ActionSubject -->|Consume| ActionHandler["Action Execute Handler"]

    %% Schema publishing
    EventHandler --> EventSchemaSubject["users.events.schema.{project_id}"]
    UserHandler --> UserSchemaSubject["users.schema.{project_id}"]
    OrgHandler --> OrgSchemaSubject["organizations.schema.{project_id}"]
    OrgUserHandler --> OrgUserSchemaSubject["organizations.users.schema.{project_id}"]
    OrgEventHandler --> OrgEventSchemaSubject["organizations.events.schema.{project_id}"]

    %% Recompute triggers from user events
    EventHandler -->|Affected Lists| ListSubject["lists.recompute.{project_id}.{list_id}"]
    EventHandler -->|Affected Journeys| JourneySubject["journeys.advance.{project_id}.{journey_id}"]

    %% Recompute triggers from users
    UserHandler -->|Affected Lists| ListSubject
    UserHandler -->|User system events| EventSubject

    %% Recompute triggers from organizations
    OrgHandler -->|Affected Lists| ListSubject
    OrgHandler -->|Org system events| OrgEventSubject
    OrgUserHandler -->|Affected Lists| ListSubject
    OrgUserHandler -->|Org user system events| OrgEventSubject

    %% Recompute triggers from organization events
    OrgEventHandler -->|Affected Lists| ListSubject
    OrgEventHandler -->|Journeys for all org users| JourneySubject

    %% Schema handlers
    EventSchemaSubject -->|Consume| EventSchemaHandler["User Event Schema Handler"]
    UserSchemaSubject -->|Consume| UserSchemaHandler["User Schema Handler"]
    OrgSchemaSubject -->|Consume| OrgSchemaHandler["Organization Schema Handler"]
    OrgUserSchemaSubject -->|Consume| OrgUserSchemaHandler["Organization User Schema Handler"]
    OrgEventSchemaSubject -->|Consume| OrgEventSchemaHandler["Organization Event Schema Handler"]

    %% Action schema publishing
    ActionHandler --> ActionSchemaSubject["actions.schema.{project_id}"]
    ActionSchemaSubject -->|Consume| ActionSchemaHandler["Action Schema Handler"]

    ListSubject -->|Consume| ListRecompute["List Recomputation"]
    JourneySubject -->|Consume| JourneyState["Journey State Handler"]

    %% Journey self-advancement and action execution
    JourneyState -->|Child Steps| JourneySubject
    JourneyState -->|Execute Action| ActionSubject
    ActionHandler -->|Reply via Inbox| JourneyState
    ActionHandler -->|Reply via Inbox| Client

    ListRecompute -->|List Membership Change| EventSubject
```
