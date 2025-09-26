import timecarder.bot


def test_bot() -> None:
    bot = timecarder.bot.MatrixBot(None, None, None, None)
    assert bot is not None
