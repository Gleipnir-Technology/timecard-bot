import asyncio
import csv
from datetime import datetime, timezone
from nio import AsyncClient, MatrixRoom, RoomMessageText
import os
from typing import Optional
import uuid

CSV_FILE = "sessions.csv"


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
        self.start_time = None
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
        print(f"[CSV LOG] {state} at {datetime.now(timezone.utc)}")  # debug print

    async def message_callback(self, room: MatrixRoom, event: RoomMessageText):
        # Only listen in the right room
        # if room.room_id != self.room_id:
        # print(f"Ignoring message from {room.room_id}")
        # return

        # Ignore bot's own messages
        if event.sender == self.client.user:
            return

        body = event.body.strip()
        if not body.startswith("!"):
            return

        # Use first word after ! as command
        command = body[1:].split()[0].upper()
        args = body.split()[1:]

        print(f"[COMMAND RECEIVED] {command} args={args}")

        if command == "START":
            if self.state == "RUNNING":
                await self.send_message("Calm down! Already running.")
            else:
                self.state = "RUNNING"
                self.start_time = datetime.now(timezone.utc)
                self.session_id = str(uuid.uuid4())
                self.log_csv("STARTED")
                await self.send_message(
                    f"Alright, off we go! Started at {self.start_time}"
                )

        elif command == "STOP":
            if self.state == "RUNNING":
                elapsed = datetime.now(timezone.utc) - self.start_time
                self.state = "STOPPED"
                self.log_csv("STOPPED")
                await self.send_message(
                    f"Stopped! Worked for {format_elapsed(elapsed)}. Bravo!"
                )
            else:
                await self.send_message(
                    "Uhh… you weren't running. Try !help if confused."
                )

        elif command == "PAUSE":
            if self.state == "RUNNING":
                elapsed = datetime.now(timezone.utc) - self.start_time
                self.state = "PAUSED"
                self.log_csv("PAUSED")
                await self.send_message(f"Paused. Worked for {format_elapsed(elapsed)}")
            else:
                await self.send_message(
                    "Nothing to pause… are you trying to pause the universe?"
                )

        elif command == "UNPAUSE":
            if self.state == "PAUSED":
                self.state = "RUNNING"
                self.start_time = datetime.now(timezone.utc)
                self.log_csv("UNPAUSED")
                await self.send_message("Resumed work. Back at it!")
            else:
                await self.send_message("Not paused. Are you making things up now?")

        elif command == "MEMO":
            self.memo = " ".join(args)
            self.log_csv("MEMO")
            await self.send_message(f"Memo saved: {self.memo} (don't lose it!)")

        elif command == "STATUS":
            await self.send_message(f"State: {self.state}, memo: {self.memo}")

        elif command == "FUSALLY":
            await self.send_message(
                "I AM YOU GOD JACK\n"
                "F YOU SALLY\n"
                "AWSOME MUSIC, and this bot is dead"
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
                room_id=room.room_id,
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
                    await self.send_message(f"Manual start at {manual_time}")
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}")
            else:
                await self.send_message("Usage: !manualstart YYYY-MM-DDTHH:MM:SS")

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
                            f"Manual stop! Worked for {format_elapsed(elapsed)}"
                        )
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}")
            else:
                await self.send_message("Usage: !manualstop YYYY-MM-DDTHH:MM:SS")

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
                            f"Manually paused! Worked for {format_elapsed(elapsed)}"
                        )
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}")
            else:
                await self.send_message("Usage: !manualpause YYYY-MM-DDTHH:MM:SS")

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
                        await self.send_message(f"Manually resumed at {manual_time}")
                    else:
                        await self.send_message("Not paused, cannot unpause manually.")
                except Exception as e:
                    await self.send_message(f"Invalid time format: {e}")
            else:
                await self.send_message("Usage: !manualunpause YYYY-MM-DDTHH:MM:SS")

        else:
            # only triggers if no command matched
            await self.send_message(
                "That sadly is not a command. Use !HELP if you… need help. I really hope I don't need to elaborate what !HELP does."
            )

    async def send_message(self, message, room_id: Optional[MatrixRoom] = None):
        room_id = room_id or self.room_id
        await self.client.room_send(
            room_id=room_id,
            message_type="m.room.message",
            content={"msgtype": "m.text", "body": message},
        )
        print(f"[SENT MESSAGE] {message}")  # debug print

    async def reminders(self):
        while True:
            await asyncio.sleep(300)
            if self.state == "RUNNING":
                await self.send_message(
                    "Hey! Are you ready to stop working yet, or are we just playing pretend?"
                )


async def run(self):
    print("[BOT STARTED] Waiting for commands…")
    await asyncio.gather(
        self.client.sync_forever(timeout=30000, full_state=True), self.reminders()
    )


async def main():
    homeserver = "https://matrix.gleipnir.technology"
    access_token = os.environ.get("ACCESS_TOKEN")
    user_id = "@timecarder:gleipnir.technology"
    room_id = os.environ.get("ROOM_ID")

    bot = MatrixBot(homeserver, access_token, user_id, room_id)
    await run(bot)


if __name__ == "__main__":
    asyncio.run(main())
