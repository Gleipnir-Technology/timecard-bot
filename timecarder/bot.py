import asyncio
import csv
from datetime import datetime, timezone
import logging
from nio import AsyncClient, Event, MatrixRoom, RoomMessageText
import os
import typing
import uuid

CSV_FILE = "sessions.csv"
LOGGER = logging.getLogger("main")


def format_elapsed(delta):
    total_seconds = int(delta.total_seconds())
    minutes, seconds = divmod(total_seconds, 60)
    return f"{minutes}m {seconds}s"


class MatrixBot:
    def __init__(self, homeserver, access_token, user_id, room_id):
        self.client = AsyncClient(homeserver, user_id)
        self.client.access_token = access_token
        self.room_id = room_id

        self.state = "IDLE"
        self.start_time = datetime.now(timezone.utc)
        self.memo = ""
        self.session_id = None

        self.client.add_event_callback(self.message_callback, RoomMessageText)

        # Ensure CSV exists
        try:
            with open(CSV_FILE, "x", newline="") as f:
                writer = csv.writer(f)
                writer.writerow(["session_id", "timestamp", "state", "memo"])
        except FileExistsError:
            pass

    def log_csv(self, state):
        with open(CSV_FILE, "a", newline="") as f:
            writer = csv.writer(f)
            writer.writerow(
                [
                    self.session_id,
                    datetime.now(timezone.utc).isoformat(),
                    state,
                    self.memo,
                ]
            )
        LOGGER.info("[CSV LOG] {%s} at {%s}", state, datetime.now(timezone.utc))

    async def message_callback(self, room: MatrixRoom, event: Event) -> None:
        event = typing.cast(RoomMessageText, event)

        # Ignore bot's own messages
        if event.sender == self.client.user:
            return

        body = event.body.strip()
        if not body.startswith("!"):
            return

        # Use first word after ! as command
        command = body[1:].split()[0].upper()
        args = body.split()[1:]
        room_id = room.room_id

        LOGGER.info("%s: %s (%s)", event.sender, command, args)

        if command == "START":
            if self.state == "RUNNING":
                await self.send_message("Calm down! Already running.", room_id)
            else:
                self.state = "RUNNING"
                self.start_time = datetime.now(timezone.utc)
                self.session_id = str(uuid.uuid4())
                self.log_csv("STARTED")
                await self.send_message(
                    f"Alright, off we go! Started at {self.start_time}", room_id
                )

        elif command == "STOP":
            if self.state == "RUNNING":
                elapsed = datetime.now(timezone.utc) - self.start_time
                self.state = "STOPPED"
                self.log_csv("STOPPED")
                await self.send_message(
                    f"Stopped! Worked for {format_elapsed(elapsed)}. Bravo!", room_id
                )
            else:
                await self.send_message(
                    "Uhh… you weren't running. Try !help if confused.", room_id
                )

        elif command == "PAUSE":
            if self.state == "RUNNING":
                elapsed = datetime.now(timezone.utc) - self.start_time
                self.state = "PAUSED"
                self.log_csv("PAUSED")
                await self.send_message(
                    f"Paused. Worked for {format_elapsed(elapsed)}", room_id
                )
            else:
                await self.send_message(
                    "Nothing to pause… are you trying to pause the universe?", room_id
                )

        elif command == "UNPAUSE":
            if self.state == "PAUSED":
                self.state = "RUNNING"
                self.start_time = datetime.now(timezone.utc)
                self.log_csv("UNPAUSED")
                await self.send_message("Resumed work. Back at it!", room_id)
            else:
                await self.send_message(
                    "Not paused. Are you making things up now?", room_id
                )

        elif command == "MEMO":
            self.memo = " ".join(args)
            self.log_csv("MEMO")
            await self.send_message(
                f"Memo saved: {self.memo} (don't lose it!)", room_id
            )

        elif command == "STATUS":
            await self.send_message(f"State: {self.state}, memo: {self.memo}", room_id)

        elif command == "FUSALLY":
            await self.send_message(
                "I AM YOU GOD JACK\n"
                "F YOU SALLY\n"
                "AWSOME MUSIC, and this bot is dead",
                room_id,
            )

        elif command == "HELP":
            await self.send_message(
                "Commands with sass and grace:\n"
                "!start – Begin working. Don’t be lazy!\n"
                "!stop – Stop the timer. Take a deep breath.\n"
                "!pause – Pause work. Snacks encouraged.\n"
                "!unpause – Resume. Pretend you weren’t gone.\n"
                "!manualstart <time> – Start at a specific time.\n"
                "!manualstop <time> – Stop at a specific time.\n"
                "!memo <text> – Save a memo.\n"
                "!status – Check your current state and memo.\n"
                "Try typing something wrong for bonus sass!",
                room.room_id,
            )

        # MANUAL COMMANDS
        elif command == "MANUALSTART":
            if args:
                try:
                    manual_time = datetime.fromisoformat(args[0])
                    if manual_time.tzinfo is None:
                        manual_time = manual_time.replace(tzinfo=timezone.utc)
                    self.start_time = manual_time
                    self.state = "RUNNING"
                    self.session_id = str(uuid.uuid4())
                    self.log_csv("MANUALSTART")
                    await self.send_message(f"Manual start at {manual_time}", room_id)
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}", room_id)
            else:
                await self.send_message(
                    "Usage: !manualstart YYYY-MM-DDTHH:MM:SS", room_id
                )

        elif command == "MANUALSTOP":
            if args:
                try:
                    manual_time = datetime.fromisoformat(args[0])
                    if manual_time.tzinfo is None:
                        manual_time = manual_time.replace(tzinfo=timezone.utc)
                    if self.start_time:
                        elapsed = manual_time - self.start_time
                        self.state = "STOPPED"
                        self.log_csv("MANUALSTOP")
                        await self.send_message(
                            f"Manual stop! Worked for {format_elapsed(elapsed)}",
                            room_id,
                        )
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}", room_id)
            else:
                await self.send_message(
                    "Usage: !manualstop YYYY-MM-DDTHH:MM:SS", room_id
                )

        elif command == "MANUALPAUSE":
            if args:
                try:
                    manual_time = datetime.fromisoformat(args[0])
                    if manual_time.tzinfo is None:
                        manual_time = manual_time.replace(tzinfo=timezone.utc)
                    if self.state == "RUNNING" and self.start_time:
                        elapsed = manual_time - self.start_time
                        self.state = "PAUSED"
                        self.log_csv("MANUALPAUSE")
                        await self.send_message(
                            f"Manually paused! Worked for {format_elapsed(elapsed)}",
                            room_id,
                        )
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}", room_id)
            else:
                await self.send_message(
                    "Usage: !manualpause YYYY-MM-DDTHH:MM:SS", room_id
                )

        elif command == "MANUALUNPAUSE":
            if args:
                try:
                    manual_time = datetime.fromisoformat(args[0])
                    if manual_time.tzinfo is None:
                        manual_time = manual_time.replace(tzinfo=timezone.utc)
                    if self.state == "PAUSED":
                        self.start_time = manual_time
                        self.state = "RUNNING"
                        self.log_csv("MANUALUNPAUSE")
                        await self.send_message(
                            f"Manually resumed at {manual_time}", room_id
                        )
                    else:
                        await self.send_message(
                            "Not paused, cannot unpause manually.", room_id
                        )
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}", room_id)
            else:
                await self.send_message(
                    "Usage: !manualunpause YYYY-MM-DDTHH:MM:SS", room_id
                )

        else:
            # only triggers if no command matched
            await self.send_message(
                "That sadly is not a command. Use !HELP if you… need help. I really hope I don't need to elaborate what !HELP does.",
                room_id,
            )

    async def send_message(self, message, room_id: str):
        room_id = room_id or self.room_id
        # await self.client.room_send(
        # room_id=room_id,
        # message_type="m.room.message",
        # content={"msgtype": "m.text", "body": message},
        # )
        LOGGER.info("[SENT MESSAGE] {%s}", message)

    async def reminders(self):
        while True:
            await asyncio.sleep(300)
            # if self.state == "RUNNING":
            # await self.send_message(
            # "Hey! Are you ready to stop working yet, or are we just playing pretend?"
            # )


async def run():
    logging.basicConfig(level=logging.INFO)
    homeserver = "https://matrix.gleipnir.technology"
    access_token = os.environ.get("BOT_TOKEN")
    user_id = "@timecarder:gleipnir.technology"
    room_id = os.environ.get("ROOM_ID")

    bot = MatrixBot(homeserver, access_token, user_id, room_id)
    LOGGER.info("[BOT STARTED] Waiting for commands…")
    await asyncio.gather(
        bot.client.sync_forever(timeout=30000, full_state=True), bot.reminders()
    )


def main():
    asyncio.run(run())


if __name__ == "__main__":
    main()
