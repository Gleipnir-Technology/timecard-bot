import os
import sys

from authlib.jose import jwt
import simplematrixbotlib as botlib


PREFIX = "!"

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
	
def main() -> None:
	header = {"alg": "HS256"}
	payload = {"sub": "timecarder"}

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
	async def echo(room, message):
		match = botlib.MessageMatch(room, message, bot, PREFIX)

		if match.is_not_from_this_bot() and match.prefix() and match.command("echo"):
			await bot.api.send_text_message(
				room.room_id, " ".join(arg for arg in match.args())
			)
	bot.run()



if __name__ == "__main__":
	main()
