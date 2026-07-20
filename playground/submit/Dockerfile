FROM python:3.11.15-slim-bookworm@sha256:b18992999dbe963a45a8a4da40ac2b1975be1a776d939d098c647482bcad5cba

WORKDIR /app

COPY sdk/python /tmp/cap-sdk
RUN pip install --no-cache-dir /tmp/cap-sdk && rm -rf /tmp/cap-sdk

COPY playground/submit /app/submit

CMD ["python", "-m", "submit.main"]
