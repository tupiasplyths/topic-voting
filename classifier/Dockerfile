FROM python:3.11-slim

ARG CLASSIFIER_MODEL=typeform/distilbert-base-uncased-mnli

WORKDIR /app

RUN python -m venv /app/.venv
ENV PATH="/app/.venv/bin:$PATH"

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

RUN python -c "from transformers import pipeline; pipeline('zero-shot-classification', model='${CLASSIFIER_MODEL}')"

COPY . .

EXPOSE 4747

CMD ["sh", "-c", "uvicorn main:app --host 0.0.0.0 --port ${CLASSIFIER_PORT:-4747}"]
