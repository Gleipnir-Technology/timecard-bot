import os
import sys
import time
from datetime import datetime

from authlib.jose import jwt
import simplematrixbotlib as botlib

PREFIX = "!"

# === Globals for time tracking ===
StartTime = 0
ElapsedTime = 0

# === Base folder for logs ===
BASE_DIR = r"C:\Users\willi\Documents\MatrixBot"

def get_token() -> str:
    token = os.environ.get("BOT_TOKEN")
    if token is None:
        try:
            with open("token.txt", "r") as f:
                token = f.read().strip()
                return token
        except OSError as e:
            print(f"Failed to read token: {e}")
        print("You must supply a BOT_TOKEN via environment variable or via token.txt")
        sys.exit(1)
    return token

def sanitize_filename(name: str) -> str:
    return "".join(c if c not in r'<>:"/\\|?*' else "_" for c in name)

def save_worklog(user_tag: str, memo: str, hours_worked: float) -> None:
    safe_tag = sanitize_filename(user_tag)
    date_str = datetime.now().strftime("%Y-%m-%d")
    folder_name = f"{safe_tag}_{date_str}"
    folder_path = os.path.join(BASE_DIR, folder_name)

    os.makedirs(folder_path, exist_ok=True)

    file_path = os.path.join(folder_path, "worklog.txt")
    text = f"Hours worked: {hours_worked:.2f}\nMemo: {memo}\n---\n"

    with open(file_path, "a", encoding="utf-8") as f:
        f.write(text)

def main() -> None:
    global StartTime, ElapsedTime

    token = get_token()
    print(f"Token: {token}")
    creds = botlib.Creds(
        homeserver="https://matrix.gleipnir.technology",
        username="timecarder",
        access_token=token,
        session_stored_file="session.txt",
    )

    bot = botlib.Bot(creds)

    @bot.listener.on_message_event
    async def command_handler(room, message):
        global StartTime, ElapsedTime

        match = botlib.MessageMatch(room, message, bot, PREFIX)
        sender = message.sender

        if not match.is_not_from_this_bot() or not match.prefix():
            return

        if match.command("echo"):
            await bot.api.send_text_message(
                room.room_id, " ".join(arg for arg in match.args())
            )

        elif match.command("test"):
            await bot.api.send_text_message(room.room_id, "🤖 I am alive! And fabulous.")

        elif match.command("whoami"):
            await bot.api.send_text_message(room.room_id, f"You're {sender}, obviously. 🙄")

        elif match.command("START"):
            StartTime = time.time()
            ElapsedTime = 0
            await bot.api.send_text_message(
                room.room_id,
                "🟢 Time tracking started. Try not to waste it like your talent. Use `!PAUSE`, `!UNPAUSE`, or `!STOP`.")

        elif match.command("PAUSE"):
            try:
                if StartTime == 0:
                    await bot.api.send_text_message(room.room_id, "😤 You can't pause time if you never started it. Try `!START` first.")
                    return
                ElapsedTime = time.time() - StartTime + ElapsedTime
                StartTime = 0
                await bot.api.send_text_message(
                    room.room_id,
                    f"⏸️ Timer paused at {ElapsedTime:.2f} seconds. Deep breaths, drama queen.")
            except Exception:
                await bot.api.send_text_message(room.room_id, "😵 Couldn't pause. Something broke and it wasn't me.")

        elif match.command("UNPAUSE"):
            try:
                if StartTime != 0:
                    await bot.api.send_text_message(room.room_id, "😬 Timer is already ticking, superstar.")
                    return
                StartTime = time.time()
                await bot.api.send_text_message(
                    room.room_id,
                    f"▶️ Timer unpaused. Back to the grind. You're at {ElapsedTime:.2f} seconds.")
            except Exception:
                await bot.api.send_text_message(room.room_id, "🙃 Couldn't unpause. Probably user error.")

        elif match.command("STOP"):
            try:
                if StartTime == 0 and ElapsedTime == 0:
                    await bot.api.send_text_message(room.room_id, "😡 Stop what? You've done *nothing*. Try `!START` first.")
                    return
                if StartTime != 0:
                    ElapsedTime = time.time() - StartTime + ElapsedTime
                    StartTime = 0
                HoursWorked = ElapsedTime / 3600
                await bot.api.send_text_message(
                    room.room_id,
                    f"⏹️ Timer stopped at {ElapsedTime:.2f} seconds ({HoursWorked:.2f} hours). Type `!MEMO` like it's a diary.")
            except Exception:
                await bot.api.send_text_message(room.room_id, "🤯 Couldn't stop timer. Honestly, how hard is it to follow instructions?")

        elif match.command("MEMO"):
            memo_text = " ".join(match.args()).strip()
            if memo_text:
                if ElapsedTime == 0:
                    await bot.api.send_text_message(
                        room.room_id,
                        "😡 There's no time to log. What exactly are you trying to memo, your thoughts?")
                else:
                    hours_worked = ElapsedTime / 3600
                    save_worklog(sender, memo_text, hours_worked)
                    ElapsedTime = 0
                    await bot.api.send_text_message(room.room_id, f"📌 Memo saved. Finally, some productivity. 📝")
            else:
                await bot.api.send_text_message(room.room_id, "🤐 Try again with actual words: `!MEMO [your brilliance]`.")

        elif match.command("HELP"):
            await bot.api.send_text_message(
                room.room_id,
                "**🆘 The Ultimete Guide to Not Being Useless**\n\n"
                "`!START` – Start that clock, baby.\n"
                "`!PAUSE` – Take a break, you've earned it (maybe).\n"
                "`!UNPAUSE` – Back to work, slacker.\n"
                "`!STOP` – Time's up. Wrap it.\n"
                "`!MEMO [text]` – Tell your manager what you did, or pretend.\n"
                "`!WHOAMI` – Existential crisis generator.\n"
                "`!ECHO [text]` – I repeat your genius.\n"
                "`!VIEW` – (Managers only) Peek behind the productivity curtain.\n"
                "`!HELP` – Because clearly you need it.")

        elif match.command("VIEW"):
            if any(name in sender for name in ["@williamribble:gleipnir.technology", "@benjaminsperry:gleipnir.technology", "@eliribble:gleipnir.technology"]):
                try:
                    summary = ""
                    logs_found = False

                    for folder_name in os.listdir(BASE_DIR):
                        folder_path = os.path.join(BASE_DIR, folder_name)
                        if os.path.isdir(folder_path):
                            file_path = os.path.join(folder_path, "worklog.txt")
                            if os.path.exists(file_path):
                                with open(file_path, "r", encoding="utf-8") as f:
                                    content = f.read().strip()
                                    if content:
                                        logs_found = True
                                        summary += (
                                            f"📁 **{folder_name}**\n"
                                            f"-------------------------\n"
                                            f"{content}\n\n"
                                        )

                    if not logs_found:
                        await bot.api.send_text_message(
                            room.room_id,
                            "🙄 Seriously? Not a single worklog? What *are* you people doing? Napping professionally?")
                    elif len(summary) < 4000:
                        await bot.api.send_text_message(room.room_id, f"🧐 Alright, here’s your sacred scroll of labor:\n\n{summary}")
                    else:
                        parts = [summary[i:i+3900] for i in range(0, len(summary), 3900)]
                        await bot.api.send_text_message(room.room_id, "📚 Buckle up. These logs are chunkier than your WiFi signal.")
                        for part in parts:
                            await bot.api.send_text_message(room.room_id, part)
                        await bot.api.send_text_message(room.room_id, "🎤 Boom. Data dropped.")
                except Exception as e:
                    await bot.api.send_text_message(
                        room.room_id,
                        f"😬 Uh-oh, I tried to read the logs but the universe said no: {e}")
            else:
                await bot.api.send_text_message(
                    room.room_id,
                    "🚫 Nope. You don't have the *vibe* to access the logs. Get promoted or get lost.")

        else:
            await bot.api.send_text_message(
                room.room_id,
                "❌ Invalid command, sweetie. Use `!HELP` before embarrassing yourself further.")

    bot.run()

if __name__ == "__main__":
    main()

