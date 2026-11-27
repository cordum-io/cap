"""A job's temp dir is still removed when a handle on its files closes just after the kill."""
import asyncio
import os
import tempfile

from cap_adapter_crewai import _proc


def test_cleanup_outlasts_a_late_handle(tmp_path):
    tmp = tempfile.mkdtemp(prefix="cap-crewai-", dir=tmp_path)
    held = open(os.path.join(tmp, "held"), "w")  # on Windows an open file blocks the first removal

    async def run():
        asyncio.get_running_loop().call_later(0.2, held.close)
        await _proc._cleanup(None, None, None, tmp, 0.0)

    try:
        asyncio.run(run())
    finally:
        held.close()
    assert not os.path.exists(tmp)
