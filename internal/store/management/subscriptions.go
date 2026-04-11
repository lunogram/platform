package management

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

const subscriptionStateUnsubscribed = 1

type Subscriptions []Subscription

func (s Subscriptions) OAPI() []oapi.Subscription {
	results := make([]oapi.Subscription, len(s))
	for i, sub := range s {
		results[i] = sub.OAPI()
	}
	return results
}

type Subscription struct {
	ID        uuid.UUID `db:"id"`
	ProjectID uuid.UUID `db:"project_id"`
	Name      string    `db:"name"`
	Channel   string    `db:"channel"`
	IsPublic  bool      `db:"is_public"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (s *Subscription) OAPI() oapi.Subscription {
	return oapi.Subscription{
		Id:        s.ID,
		ProjectId: s.ProjectID,
		Name:      s.Name,
		Channel:   oapi.Channel(s.Channel),
		IsPublic:  s.IsPublic,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

type UserSubscriptions []UserSubscription

func (us UserSubscriptions) OAPI() []oapi.UserSubscription {
	results := make([]oapi.UserSubscription, len(us))
	for i, sub := range us {
		results[i] = sub.OAPI()
	}
	return results
}

type UserSubscription struct {
	SubscriptionID uuid.UUID `db:"subscription_id"`
	Name           string    `db:"name"`
	Channel        string    `db:"channel"`
	State          string    `db:"state"`
}

func (us *UserSubscription) OAPI() oapi.UserSubscription {
	return oapi.UserSubscription{
		SubscriptionId: us.SubscriptionID,
		Name:           us.Name,
		Channel:        oapi.Channel(us.Channel),
		State:          oapi.SubscriptionState(us.State),
	}
}

func NewSubscriptionsStore(db store.DB) *SubscriptionsStore {
	return &SubscriptionsStore{db: db}
}

type SubscriptionsStore struct {
	db store.DB
}

func (s *SubscriptionsStore) GetUserSubscriptions(ctx context.Context, projectID, userID uuid.UUID, pagination store.Pagination) (UserSubscriptions, int, error) {
	query := `
	SELECT
		s.id AS subscription_id,
		s.name,
		s.channel,
		CASE WHEN us.state = $5 THEN 'unsubscribed' ELSE 'subscribed' END AS state,
		COUNT(*) OVER () AS total_count
	FROM subscriptions s
	LEFT JOIN user_subscription us ON us.subscription_id = s.id AND us.user_id = $2
	WHERE s.project_id = $1 AND s.is_public = true
	ORDER BY s.name
	LIMIT $3 OFFSET $4`

	type result struct {
		SubscriptionID uuid.UUID `db:"subscription_id"`
		Name           string    `db:"name"`
		Channel        string    `db:"channel"`
		State          string    `db:"state"`
		TotalCount     int       `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, userID, pagination.Limit, pagination.Offset, subscriptionStateUnsubscribed)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []UserSubscription{}, 0, nil
	}

	total := results[0].TotalCount
	subscriptions := make([]UserSubscription, len(results))
	for i, r := range results {
		subscriptions[i] = UserSubscription{
			SubscriptionID: r.SubscriptionID,
			Name:           r.Name,
			Channel:        r.Channel,
			State:          r.State,
		}
	}

	return subscriptions, total, nil
}

func (s *SubscriptionsStore) GetAllUserSubscriptions(ctx context.Context, projectID, userID uuid.UUID) (UserSubscriptions, error) {
	query := `
	SELECT
		s.id AS subscription_id,
		s.name,
		s.channel,
		CASE WHEN us.state = $3 THEN 'unsubscribed' ELSE 'subscribed' END AS state
	FROM subscriptions s
	LEFT JOIN user_subscription us ON us.subscription_id = s.id AND us.user_id = $2
	WHERE s.project_id = $1 AND s.is_public = true
	ORDER BY s.name`

	var subscriptions []UserSubscription
	err := s.db.SelectContext(ctx, &subscriptions, query, projectID, userID, subscriptionStateUnsubscribed)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *SubscriptionsStore) GetSubscription(ctx context.Context, projectID, subscriptionID uuid.UUID) (*Subscription, error) {
	stmt := `
	SELECT id, project_id, name, channel, is_public, created_at, updated_at
	FROM subscriptions
	WHERE id = $1 AND project_id = $2`

	var subscription Subscription
	err := s.db.GetContext(ctx, &subscription, stmt, subscriptionID, projectID)
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

func (s *SubscriptionsStore) CreateSubscription(ctx context.Context, subscription Subscription) (uuid.UUID, error) {
	stmt := `
	INSERT INTO subscriptions (project_id, name, channel, is_public)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		subscription.ProjectID,
		subscription.Name,
		subscription.Channel,
		subscription.IsPublic,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *SubscriptionsStore) Subscribe(ctx context.Context, userID, subscriptionID uuid.UUID) error {
	stmt := `
	DELETE FROM user_subscription
	WHERE user_id = $1 AND subscription_id = $2`

	_, err := s.db.ExecContext(ctx, stmt, userID, subscriptionID)
	return err
}

func (s *SubscriptionsStore) Unsubscribe(ctx context.Context, userID, subscriptionID uuid.UUID) error {
	// Delete any existing record first
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM user_subscription
		WHERE user_id = $1 AND subscription_id = $2`, userID, subscriptionID)
	if err != nil {
		return err
	}

	// Insert unsubscribe record
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_subscription (user_id, subscription_id, state, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())`, userID, subscriptionID, subscriptionStateUnsubscribed)
	return err
}

func (s *SubscriptionsStore) SetSubscriptionState(ctx context.Context, userID, subscriptionID uuid.UUID, subscribed bool) error {
	if subscribed {
		return s.Subscribe(ctx, userID, subscriptionID)
	}
	return s.Unsubscribe(ctx, userID, subscriptionID)
}

func (s *SubscriptionsStore) IsUserUnsubscribed(ctx context.Context, userID, subscriptionID uuid.UUID) (bool, error) {
	var state *int
	err := s.db.GetContext(ctx, &state,
		`SELECT state FROM user_subscription WHERE user_id = $1 AND subscription_id = $2`,
		userID, subscriptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return state != nil && *state == subscriptionStateUnsubscribed, nil
}

func (s *SubscriptionsStore) ListSubscriptions(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (Subscriptions, int, error) {
	query := `
	SELECT
		id,
		project_id,
		name,
		channel,
		is_public,
		created_at,
		updated_at,
		COUNT(*) OVER () AS total_count
	FROM subscriptions
	WHERE project_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		Subscription
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Subscription{}, 0, nil
	}

	total := results[0].TotalCount
	subscriptions := make([]Subscription, len(results))

	for index, r := range results {
		subscriptions[index] = r.Subscription
	}

	return subscriptions, total, nil
}

func (s *SubscriptionsStore) UpdateSubscription(ctx context.Context, subscriptionID uuid.UUID, name string, isPublic bool) error {
	stmt := `
	UPDATE subscriptions
	SET name = $1, is_public = $2
	WHERE id = $3`

	_, err := s.db.ExecContext(ctx, stmt, name, isPublic, subscriptionID)
	return err
}
