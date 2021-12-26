-- DELETE FROM hist WHERE time_tag < (now() - '30 days'::interval);
SELECT drop_chunks('hist', INTERVAL '30 days');
