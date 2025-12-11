import asyncio
from typing import Optional

import nats


class NATSConfig:
    def __init__(
        self,
        url: str,
        token: Optional[str] = None,
        username: Optional[str] = None,
        password: Optional[str] = None,
        name: str = "cap-sdk-python",
    ):
        self.url = url
        self.token = token
        self.username = username
        self.password = password
        self.name = name


async def connect_nats(cfg: NATSConfig):
    opts = {"servers": cfg.url, "name": cfg.name}
    if cfg.token:
        opts["token"] = cfg.token
    if cfg.username:
        opts["user"] = cfg.username
    if cfg.password:
        opts["password"] = cfg.password
    return await nats.connect(**opts)
