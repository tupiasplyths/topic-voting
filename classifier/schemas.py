from pydantic import BaseModel, Field


class ClassifyRequest(BaseModel):
    message: str = Field(..., min_length=1, max_length=512)
    topic: str = Field(..., min_length=1, max_length=255)
    topic_id: str | None = Field(None, max_length=255, pattern=r'^[a-zA-Z0-9_-]+$')
    existing_labels: list[str] = Field(default_factory=list)
    threshold: float = Field(default=0.5, ge=0.0, le=1.0)


class ClassifyResponse(BaseModel):
    label: str
    confidence: float
    is_new: bool
    all_scores: dict[str, float] | None = None
