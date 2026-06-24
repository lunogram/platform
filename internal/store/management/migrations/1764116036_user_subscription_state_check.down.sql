ALTER TABLE user_subscription
    DROP CONSTRAINT IF EXISTS user_subscription_state_check;

ALTER TABLE user_subscription
    ALTER COLUMN state TYPE SMALLINT
    USING (
        CASE
            WHEN state = 'unsubscribed' THEN 1
            ELSE NULL
        END
    );
