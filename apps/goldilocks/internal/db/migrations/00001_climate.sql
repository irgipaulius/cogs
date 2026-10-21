-- +goose Up
CREATE TABLE meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE cells (
  level INTEGER NOT NULL,
  lat_i INTEGER NOT NULL,
  lon_i INTEGER NOT NULL,
  temp_c BLOB NOT NULL,
  precip_mm BLOB NOT NULL,
  cloud_pct BLOB,
  sunny_days INTEGER,
  year_count INTEGER NOT NULL DEFAULT 10,
  PRIMARY KEY (level, lat_i, lon_i)
);

CREATE INDEX idx_cells_level_lat_lon ON cells (level, lat_i, lon_i);

CREATE TABLE cell_countries (
  level INTEGER NOT NULL,
  lat_i INTEGER NOT NULL,
  lon_i INTEGER NOT NULL,
  country TEXT NOT NULL,
  PRIMARY KEY (level, lat_i, lon_i)
);

CREATE INDEX idx_cell_countries_country ON cell_countries (country, level);

CREATE TABLE scrape_cursor (
  source TEXT NOT NULL,
  cursor_key TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (source, cursor_key)
);

-- +goose Down
DROP TABLE scrape_cursor;
DROP TABLE cell_countries;
DROP TABLE cells;
DROP TABLE meta;
