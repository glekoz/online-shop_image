-- +goose Up
-- +goose StatementBegin
CREATE TABLE entity_state (
    service VARCHAR(50) NOT NULL,
    entity_id VARCHAR(100) NOT NULL,
    image_count INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,
    max_count INTEGER NOT NULL,
    PRIMARY KEY (service_name, entity_id)
) PARTITION BY LIST (service_name);

CREATE INDEX entity_id_idx ON entity_state (entity_id);

CREATE TABLE user_state PARTITION OF entity_state
FOR VALUES IN ('user');

CREATE TABLE product_state PARTITION OF entity_state
FOR VALUES IN ('product');


CREATE TABLE entity_image_list (
    service VARCHAR(50) NOT NULL,
    entity_id VARCHAR(100) NOT NULL,
    image_path VARCHAR(200) UNIQUE NOT NULL,
    is_cover BOOLEAN NOT NULL,
    PRIMARY KEY (service_name, entity_id),
    FOREIGN KEY (service_name, entity_id) 
        REFERENCES entity_state(service_name, entity_id)
        ON DELETE CASCADE
) PARTITION BY LIST (service_name);

CREATE TABLE user_image_list PARTITION OF entity_image_list
FOR VALUES IN ('user');

CREATE TABLE product_image_list PARTITION OF entity_image_list
FOR VALUES IN ('product');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_state;
DROP TABLE product_state;
DROP TABLE entity_state;
DROP INDEX entity_id_idx;

DROP TABLE user_image_list;
DROP TABLE product_image_list;
DROP TABLE entity_image_list;
-- +goose StatementEnd
