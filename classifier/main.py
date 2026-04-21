import asyncio
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException

from classifier import VoteClassifier
from schemas import ClassifyRequest, ClassifyResponse

logger = logging.getLogger(__name__)

classifier: VoteClassifier | None = None
semaphore: asyncio.Semaphore | None = None

MAX_CONCURRENT_INFERENCES = 4


@asynccontextmanager
async def lifespan(app: FastAPI):
    global classifier, semaphore
    classifier = VoteClassifier()
    semaphore = asyncio.Semaphore(MAX_CONCURRENT_INFERENCES)
    yield
    classifier = None
    semaphore = None


app = FastAPI(title="Topic Voting Classifier", lifespan=lifespan)


@app.post("/classify", response_model=ClassifyResponse)
async def classify(req: ClassifyRequest):
    if classifier is None:
        raise HTTPException(status_code=503, detail="model_not_loaded")

    try:
        async with semaphore:
            result = await asyncio.to_thread(classifier.classify, req)
        if req.topic_id:
            classifier.register_label(req.topic_id, result.label)
        return result
    except Exception as e:
        logger.error(f"Classification error: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail="classification_error")


@app.get("/health")
async def health():
    return {
        "status": "healthy" if classifier is not None else "unhealthy",
        "model_loaded": classifier is not None,
        "model_name": classifier.model_name if classifier is not None else None,
    }


@app.get("/labels")
async def labels(topic_id: str):
    if classifier is None:
        return {"topic_id": topic_id, "labels": []}
    return {"topic_id": topic_id, "labels": classifier.get_labels(topic_id)}