def log(message: str) -> None:
    """
    Send message over unbuffered stdout for RHC to log
    """
    print(message + "\n", flush=True)
