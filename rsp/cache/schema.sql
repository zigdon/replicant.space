PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS stars (
  designation TEXT PRIMARY KEY,
  name TEXT,
  entry_point TEXT,
  est_planets INTEGER,
  explored INTEGER,
  has_life INTEGER,
  position_x REAL,
  position_y REAL,
  position_z REAL
);

CREATE TABLE IF NOT EXISTS planets (
  designation TEXT PRIMARY KEY,
  star TEXT NOT NULL,
  name TEXT,
  life_stage TEXT,
  moons INTEGER,
  rings INTEGER,
  scanned INTEGER,
  type INTEGER,
  FOREIGN KEY(star) REFERENCES stars(designation)
);

CREATE TABLE IF NOT EXISTS moons (
  designation TEXT PRIMARY KEY,
  planet TEXT NOT NULL,
  star TEXT NOT NULL,
  name TEXT,
  scanned INTEGER,
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

CREATE TABLE IF NOT EXISTS resources (
  belt TEXT NOT NULL,
  resource TEXT,
  density TEXT,
  FOREIGN KEY(belt) REFERENCES belts(designation)
);
