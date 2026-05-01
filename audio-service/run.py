import argparse
import uvicorn

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, default=8765)
    parser.add_argument("--host", default="127.0.0.1")
    args = parser.parse_args()
    uvicorn.run("main:app", host=args.host, port=args.port, log_level="warning")
