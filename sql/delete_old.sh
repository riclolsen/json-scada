#!/bin/bash

psql -U postgres -h 127.0.0.1 json_scada < delete_old.sql

