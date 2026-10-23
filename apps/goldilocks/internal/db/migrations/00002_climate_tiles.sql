-- +goose Up
CREATE TABLE climate_tiles (
  level INTEGER NOT NULL,
  sy INTEGER NOT NULL,
  sx INTEGER NOT NULL,
  cells INTEGER NOT NULL DEFAULT 0,
  bytes BLOB NOT NULL,
  PRIMARY KEY (level, sy, sx)
);

CREATE INDEX idx_climate_tiles_level ON climate_tiles (level);

-- +goose Down
DROP TABLE climate_tiles;
