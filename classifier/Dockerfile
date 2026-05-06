FROM python:3.11-slim AS builder

ARG CLASSIFIER_MODEL=typeform/distilbert-base-uncased-mnli

WORKDIR /app

RUN python -m venv /app/.venv
ENV PATH="/app/.venv/bin:$PATH"

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Pre-download model at build time
RUN python -c "from transformers import pipeline; pipeline('zero-shot-classification', model='${CLASSIFIER_MODEL}')"

# Runtime stage
FROM python:3.11-slim

ARG CLASSIFIER_PORT=4747
ENV CLASSIFIER_PORT=${CLASSIFIER_PORT}

WORKDIR /app

# Copy venv from builder
COPY --from=builder /app/.venv /app/.venv
ENV PATH="/app/.venv/bin:$PATH"

# Copy application code
COPY . .

EXPOSE ${CLASSIFIER_PORT}

CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port ${CLASSIFIER_PORT}"]
