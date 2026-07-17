PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS stars (
  designation TEXT PRIMARY KEY,
  name TEXT,
  entry_point TEXT,
  est_planets INTEGER,
  spectral_type TEXT NOT NULL,
  explored INTEGER NOT NULL,
  has_life INTEGER NOT NULL,
  position_x REAL,
  position_y REAL,
  position_z REAL,
  has_hub INTEGER NOT NULL,
  has_my_hub INTEGER NOT NULL,
  entry_point TEXT
);

CREATE TABLE IF NOT EXISTS planets (
  designation TEXT PRIMARY KEY,
  star TEXT NOT NULL,
  name TEXT,
  life_stage TEXT,
  moons INTEGER,
  rings INTEGER,
  scanned INTEGER NOT NULL,
  type INTEGER,
  FOREIGN KEY(star) REFERENCES stars(designation)
);

CREATE TABLE IF NOT EXISTS moons (
  designation TEXT PRIMARY KEY,
  planet TEXT NOT NULL,
  star TEXT NOT NULL,
  name TEXT,
  scanned INTEGER NOT NULL,
  type TEXT,
  FOREIGN KEY(planet) REFERENCES planets(designation),
  FOREIGN KEY(star) REFERENCES stars(designation)
);

CREATE TABLE IF NOT EXISTS belts (
  designation TEXT PRIMARY KEY,
  star TEXT NOT NULL,
  density TEXT,
  FOREIGN KEY(star) REFERENCES stars(designation)
);

CREATE TABLE IF NOT EXISTS belt_resources (
  belt TEXT NOT NULL,
  resource TEXT,
  density TEXT,
  FOREIGN KEY(belt) REFERENCES belts(designation)
);

CREATE TABLE IF NOT EXISTS aliases (
  designation TEXT PRIMARY KEY,
  type TEXT,
  name TEXT UNIQUE
);

CREATE TABLE IF NOT EXISTS alias_types (
  type TEXT PRIMARY KEY,
  prefix TEXT UNIQUE
);

CREATE TABLE IF NOT EXISTS blueprints (
  type TEXT PRIMARY KEY,
  print_time REAL,
  attach_capacity INTEGER,
  cargo_capacity INTEGER,
  stow_capacity INTEGER,
  short TEXT,
  description TEXT
);

CREATE TABLE IF NOT EXISTS blueprint_resources(
  blueprint_type TEXT NOT NULL,
  type TEXT,
  qty INTEGER,
  FOREIGN KEY(blueprint_type) REFERENCES blueprints(type),
  UNIQUE (blueprint_type, type) ON CONFLICT REPLACE
);

CREATE TABLE IF NOT EXISTS blueprint_components(
  blueprint_type TEXT NOT NULL,
  type TEXT,
  qty INTEGER,
  FOREIGN KEY(blueprint_type) REFERENCES blueprints(type),
  UNIQUE (blueprint_type, type) ON CONFLICT REPLACE
);

CREATE TABLE IF NOT EXISTS blueprint_directives(
  blueprint_type TEXT NOT NULL,
  directive TEXT,
  FOREIGN KEY(blueprint_type) REFERENCES blueprints(type),
  UNIQUE (blueprint_type, directive) ON CONFLICT REPLACE
);

CREATE TABLE IF NOT EXISTS blueprint_features(
  blueprint_type TEXT NOT NULL,
  feature TEXT,
  FOREIGN KEY(blueprint_type) REFERENCES blueprints(type),
  UNIQUE (blueprint_type, feature) ON CONFLICT REPLACE
);

CREATE TABLE IF NOT EXISTS notifications(
  id INTEGER PRIMARY KEY,
  start INTEGER,
  end INTEGER,
  device TEXT,
  text TEXT,
  read INTEGER
);

CREATE TABLE IF NOT EXISTS messages(
  id INTEGER PRIMARY KEY,
  body TEXT,
  created INTEGER,
  read INTEGER,
  type TEXT,
  title TEXT
);

CREATE TABLE IF NOT EXISTS cached_journey(
  id INTEGER PRIMARY KEY,
  start TEXT NOT NULL,
  end TEXT NOT NULL,
  max_hop REAL,
  calculated INTEGER,
  UNIQUE (start, end, max_hop) ON CONFLICT REPLACE
);

CREATE TABLE IF NOT EXISTS cached_journey_steps(
  journey_id INTEGER,
  src TEXT NOT NULL,
  dest TEXT NOT NULL,
  step INTEGER,
  dist_src float32,
  dist_dest float32,
  FOREIGN KEY(journey_id) REFERENCES cached_routes(id)
  UNIQUE (journey_id, step) ON CONFLICT REPLACE
);
