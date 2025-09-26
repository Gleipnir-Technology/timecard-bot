import asyncio
from datetime import datetime, timezone
import time
from typing import Dict, List

from nio import MatrixRoom, RoomMessageText

from timecarder import bot


TEST_USER = "test-user-id"


class BotForTesting(bot.MatrixBot):
    def __init__(self):
        self.messages: Dict[str, List[str]] = {}
        self.own_user_id = "fake-own-user-id"
        self.room_id = "fake-user-id"
        self.room = MatrixRoom(self.room_id, self.own_user_id)
        super().__init__(None, None, None, None)

    def send(self, message: str) -> None:
        source = {
            "event_id": None,
            "sender": "test-user-id",
            "origin_server_ts": time.time() * 1000,
        }
        text = RoomMessageText(
            body=message, formatted_body=None, format=None, source=source
        )
        asyncio.run(self.message_callback(self.room, text))

    async def send_message(self, message: str, room_id: str):
        messages = self.messages.get(room_id, [])
        messages.append(message)
        self.messages[room_id] = messages


def test_bot() -> None:
    b = BotForTesting()
    b.send("!manualstart 2000-09-05T01:00:00")
    session = b.sessions[TEST_USER]
    assert session.state == "RUNNING"
    assert session.started == datetime(2000, 9, 5, 1, 0, 0, tzinfo=timezone.utc)
