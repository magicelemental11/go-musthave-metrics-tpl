CREATE TABLE metrics (
    id    text NOT NULL,
    type  text NOT NULL CHECK (type IN ('gauge', 'counter')),
    delta bigint,
    value double precision,
    PRIMARY KEY (id, type),
    CHECK (
        (type = 'gauge'   AND value IS NOT NULL AND delta IS NULL) OR
        (type = 'counter' AND delta IS NOT NULL AND value IS NULL)
    )
);
