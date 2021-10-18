
-- psql -h 127.0.0.1 -U postgres -w -f create_tables.sql

CREATE DATABASE "json_scada"
    WITH OWNER "postgres"
    ENCODING 'UTF8'
    -- LC_COLLATE = 'en-US.UTF8' -- can cause errors sometimes
    -- LC_CTYPE = 'en-US.UTF8' 
    TEMPLATE template0;

\c json_scada

CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- disable timescaledb telemetry
ALTER SYSTEM SET timescaledb.telemetry_level=off;

-- create tables

-- DROP TABLE hist;
CREATE TABLE IF NOT EXISTS hist (
   tag text not null,
   time_tag TIMESTAMPTZ(3),
   value float not null,
   value_json jsonb,
   time_tag_at_source TIMESTAMPTZ(3),
   flags bit(8) not null,
   PRIMARY KEY ( tag, time_tag )
   );
CREATE INDEX ind_timeTag on hist ( time_tag );
CREATE INDEX ind_tagTimeTag on hist ( tag, time_tag_at_source );
comment on table hist is 'Historical data table';
comment on column hist.tag is 'String key for the point';
comment on column hist.value is 'Value as a double';
comment on column hist.time_tag is 'GMT Timestamp for the time data was received by the server';
comment on column hist.time_tag_at_source is 'Field GMT timestamp for the event (null if not available)';
comment on column hist.value_json is 'Structured value as JSON, can be null when do not apply. For digital point it should be the status as in {s:"OFF"}';
comment on column hist.flags is 'Bit mask 0x80=value invalid, 0x40=Time tag at source invalid, 0x20=Analog, 0x10=value recorded by integrity (not by variation)';

-- timescaledb hypertable, partitioned by day
SELECT create_hypertable('hist', 'time_tag', chunk_time_interval=>86400000000);
-- data retention policy (older data will be deleted)

-- SELECT add_drop_chunks_policy('hist', INTERVAL '45 days'); -- this is for timescaledb < 2.0
SELECT add_retention_policy('hist', INTERVAL '45 days');

-- DROP TABLE realtime_data;
CREATE TABLE IF NOT EXISTS realtime_data (
   tag text not null,
   time_tag TIMESTAMPTZ(3) not null,
   json_data jsonb,
   PRIMARY KEY ( tag )
   );

comment on table realtime_data is 'Realtime data and catalog data';
comment on column realtime_data.tag is 'String key for the point';
comment on column realtime_data.time_tag is 'GMT Timestamp for the data update';
comment on column realtime_data.json_data is 'Data image as JSON from Mongodb';
CREATE INDEX ind_tag on realtime_data ( tag );


-- drop view grafana_hist;
create view grafana_hist as
SELECT
  time_tag AS "time",
  tag AS metric,
  value as value,
  time_tag_at_source,
  value_json,
  flags
FROM hist;

-- drop view grafana_realtime;
create view grafana_realtime as
select 
tag as metric, 
time_tag as time, 
cast(json_data->>'value' as float) as value, 
cast(json_data->>'_id' as text) as point_key, 
json_data->>'valueString' as value_string, 
cast(json_data->>'invalid' as boolean) as invalid, 
json_data->>'description' as description, 
json_data->>'group1' as group1  
from realtime_data;

-- In porstgresql create a grafana user just for selects
CREATE USER grafana WITH PASSWORD 'JSGrafana';
GRANT CONNECT ON DATABASE json_scada TO grafana;
GRANT USAGE ON SCHEMA public TO grafana;
GRANT SELECT ON hist TO grafana;
GRANT SELECT ON realtime_data TO grafana;
GRANT SELECT ON grafana_hist TO grafana;
GRANT SELECT ON grafana_realtime TO grafana;

CREATE USER json_scada WITH PASSWORD 'json_scada';
GRANT CONNECT ON DATABASE json_scada TO json_scada;
GRANT all ON realtime_data, hist, grafana_hist TO json_scada;
