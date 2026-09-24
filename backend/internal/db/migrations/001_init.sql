-- Esquema inicial. Timestamps sin zona horaria: el CSV trae hora local de planta (ver docs/DESIGN.md).

CREATE TABLE meters (
    id          BIGSERIAL PRIMARY KEY,
    meter_id    TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    location    TEXT,
    status      TEXT        NOT NULL DEFAULT 'ACTIVE',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE readings (
    id               BIGSERIAL PRIMARY KEY,
    meter_id         TEXT             NOT NULL REFERENCES meters(meter_id),
    ts               TIMESTAMP        NOT NULL,
    consumption_kwh  DOUBLE PRECISION NOT NULL,
    voltage_v        DOUBLE PRECISION NOT NULL,
    current_a        DOUBLE PRECISION NOT NULL,
    power_factor     DOUBLE PRECISION NOT NULL,
    status           TEXT             NOT NULL,
    UNIQUE (meter_id, ts) -- evita duplicados y sirve de índice para consultas por rango
);

CREATE TABLE events (
    id           BIGSERIAL PRIMARY KEY,
    meter_id     TEXT      NOT NULL REFERENCES meters(meter_id),
    ts           TIMESTAMP NOT NULL,
    type         TEXT      NOT NULL,
    description  TEXT      NOT NULL
);
CREATE INDEX events_meter_ts_idx ON events (meter_id, ts);

-- Cola de trabajos: el worker toma filas PENDING con FOR UPDATE SKIP LOCKED.
CREATE TABLE analysis_runs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    status        TEXT        NOT NULL DEFAULT 'PENDING'
                  CHECK (status IN ('PENDING','RUNNING','COMPLETED','FAILED')),
    current_step  TEXT,
    error         TEXT,
    summary       JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);
CREATE INDEX analysis_runs_pending_idx ON analysis_runs (created_at) WHERE status = 'PENDING';

CREATE TABLE anomalies (
    id                  UUID             PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id         UUID             NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    meter_id            TEXT             NOT NULL REFERENCES meters(meter_id),
    detected_at         TIMESTAMP        NOT NULL,
    type                TEXT             NOT NULL
                        CHECK (type IN ('REAL_ANOMALY','EXPLAINABLE_ANOMALY','FALSE_POSITIVE','DATA_QUALITY')),
    severity            TEXT             NOT NULL CHECK (severity IN ('LOW','MEDIUM','HIGH')),
    confidence          DOUBLE PRECISION NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    priority_score      DOUBLE PRECISION NOT NULL,
    reason              TEXT             NOT NULL,
    recommended_action  TEXT             NOT NULL,
    evidence            JSONB            NOT NULL,
    status              TEXT             NOT NULL DEFAULT 'OPEN'
                        CHECK (status IN ('OPEN','ACKNOWLEDGED','RESOLVED')),
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT now()
);
CREATE INDEX anomalies_analysis_idx ON anomalies (analysis_id);
CREATE INDEX anomalies_meter_idx ON anomalies (meter_id);
