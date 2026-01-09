import os
import sys

# Allow generated stubs to resolve `cordum.*` absolute imports.
_pb_root = os.path.dirname(__file__)
if _pb_root not in sys.path:
    sys.path.insert(0, _pb_root)
