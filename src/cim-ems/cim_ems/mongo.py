"""MongoDB access: connection, collections, indexes and the GridFS artifact bucket."""
from __future__ import annotations

import gridfs
from pymongo import ASCENDING, MongoClient
from pymongo.database import Database

from .config import MainConfig

MODELS = "cimModels"
BINDINGS = "cimMeasurementBindings"
RUNS = "cimRuns"
CONTINGENCY_LISTS = "cimContingencyLists"
STUDY_REQUESTS = "cimStudyRequests"
ARTIFACT_BUCKET = "cimArtifacts"

RUNS_TTL_SECONDS = 7 * 24 * 3600


def connect(cfg: MainConfig) -> Database:
    return MongoClient(cfg.mongo_uri, appname="CIM-EMS")[cfg.database]


def ensure_indexes(db: Database, runs_ttl_seconds: int = RUNS_TTL_SECONDS) -> None:
    db[MODELS].create_index([("modelId", ASCENDING), ("revision", ASCENDING)], unique=True, name="cimModelsIndex")
    # At most one active revision per model.
    db[MODELS].create_index(
        [("modelId", ASCENDING)],
        unique=True,
        partialFilterExpression={"status": "active"},
        name="cimModelsActiveIndex",
    )
    db[BINDINGS].create_index(
        [("modelId", ASCENDING), ("bindingRevision", ASCENDING), ("cimMeasurementMrid", ASCENDING)],
        name="cimBindingsIndex",
    )
    db[RUNS].create_index([("connectionNumber", ASCENDING), ("kind", ASCENDING), ("runId", ASCENDING)], name="cimRunsIndex")
    db[RUNS].create_index([("createdAt", ASCENDING)], expireAfterSeconds=runs_ttl_seconds, name="cimRunsTtl")
    db[CONTINGENCY_LISTS].create_index([("listId", ASCENDING)], unique=True, name="cimContingencyListsIndex")
    db[STUDY_REQUESTS].create_index([("status", ASCENDING), ("createdAt", ASCENDING)], name="cimStudyRequestsIndex")


def artifacts(db: Database) -> gridfs.GridFS:
    return gridfs.GridFS(db, collection=ARTIFACT_BUCKET)
