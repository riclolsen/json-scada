import mongomock
import mongomock.gridfs
from mongomock.collection import BulkOperationBuilder

mongomock.gridfs.enable_gridfs_integration()

# pymongo >= 4.12 passes `sort` to add_update; mongomock does not know it yet.
_orig = BulkOperationBuilder.add_update


def _add_update(self, *args, sort=None, **kwargs):
    return _orig(self, *args, **kwargs)


BulkOperationBuilder.add_update = _add_update
