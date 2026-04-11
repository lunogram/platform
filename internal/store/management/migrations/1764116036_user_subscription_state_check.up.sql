ALTER TABLE user_subscription
    ALTER COLUMN state TYPE VARCHAR(64)
    USING (
        CASE
            WHEN state = 1 THEN 'unsubscribed'
            ELSE NULL
        END
    );

ALTER TABLE user_subscription
    ADD CONSTRAINT user_subscription_state_check
    CHECK (state IN ('unsubscribed'));
