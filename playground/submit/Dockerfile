FROM python:3.11-slim

WORKDIR /app

# Install the CAP Python SDK from the repo
COPY sdk/python /tmp/cap-sdk
RUN pip install --no-cache-dir /tmp/cap-sdk && rm -rf /tmp/cap-sdk

COPY playground/submit/main.py .

CMD ["python", "main.py"]
