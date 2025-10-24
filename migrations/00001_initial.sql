-- +goose Up
CREATE TABLE user_ (
	ID SERIAL PRIMARY KEY,
	display_name VARCHAR(200),
	matrix_userid TEXT
);

CREATE TABLE task (
	ID SERIAL PRIMARY KEY,
	description TEXT,
	started TIMESTAMP NOT NULL,
	stopped TIMESTAMP NOT NULL,
	user_id NOT NULL REFERENCES
);

-- +goose Down
DROP TABLE task;
DROP TABLE user_;
